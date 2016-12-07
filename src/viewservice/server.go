package viewservice

import (
	"sync"
	"net"
	"fmt"
	"sync/atomic"
	"net/rpc"
	"os"
	"log"
	"time"
)

type ViewServer struct {
	mu sync.Mutex
	listener net.Listener
	dead int32
	rpcCount int32
	me string

	currentView *View
	newView *View
	primaryAcked bool //当前view是否被view的primary acked
	primaryTTL uint
	backupTTL uint
	idleServers map[string]int // string -> ttl
}

func createView(viewNum uint, primary string, backup string) (view *View) {
	view = new(View)
	view.Viewnum = viewNum
	view.Primary = primary
	view.BackUp  = backup
	return
}

func (vs *ViewServer) printView()  {
	if vs.currentView == nil {
		fmt.Println("No view in the view server!")
	} else {
		fmt.Printf("Current view:%d, %s, %s\n", vs.currentView.Viewnum, vs.currentView.Primary, vs.currentView.BackUp)
	}
}

func (vs *ViewServer) doSwitch() (bool)  {
	if vs.newView != nil {
		vs.currentView, vs.newView = vs.newView, nil
		return true
	}
	return false
}

func (vs *ViewServer) updateNewView(primary string, backup string)  {
	if vs.currentView == nil {return }
	if vs.newView == nil {
		vs.newView = createView(vs.currentView.Viewnum + 1, primary, backup)
	} else {
		vs.newView.Primary = primary
		vs.newView.BackUp = backup
	}
}

func (vs *ViewServer) getAndDel() (string) {
	for server,_ := range vs.idleServers {
		delete(vs.idleServers, server)
		return server
	}
	return ""
}

func (vs *ViewServer) updateAndSwitch() (bool) {
	view := vs.currentView
	if view.BackUp == "" && len(vs.idleServers) == 0 {
		return false
	}
	if vs.primaryTTL > 0 && vs.backupTTL <= 0 {
		vs.updateNewView(vs.currentView.Primary, vs.getAndDel())
	} else if vs.primaryTTL <= 0 && vs.backupTTL > 0 {
		vs.updateNewView(vs.currentView.BackUp, vs.getAndDel())
	} else if vs.primaryTTL <= 0 && vs.backupTTL <= 0 {
		//由于db基于内存的，没有同步过的idle server不能直接作为primary
		vs.updateNewView("", "")
	}
	return vs.doSwitch()
}

func (vs *ViewServer) Ping(args *PingArgs, reply *PingReply) error  {
	vs.mu.Lock()
	defer vs.mu.Unlock()

	pingServer, pingViewNum := args.Me, args.ViewNum
	fmt.Printf("Ping message from %v %d\n", pingServer, pingViewNum)
	vs.printView()
	if pingViewNum == 0 {
		//如果刚加入集群的话
		if vs.currentView == nil {
			//如果view server刚启动
			fmt.Printf("Begin to create new view %s %d\n",pingServer, pingViewNum)
			vs.currentView = createView(1, pingServer, "")
			fmt.Printf("Create view success %+v \n", vs.currentView)
		} else {
			//down -> active
			if vs.currentView.Primary == pingServer {
				//新的view还没有被acked
				fmt.Println("Primary restarted!")
				vs.primaryTTL = 0 //刚启动不能作为primary
				//如果primary已经ack当前view，那么make new view
				//这么做是为了保证每次view的序号加一，避免维护多个版本保持一致性
				if vs.primaryAcked && vs.updateAndSwitch(){
					vs.primaryAcked = false
				}
			}

			if pingServer != "" && pingServer != vs.currentView.BackUp {
				vs.idleServers[pingServer] = DeadPings
			}
		}
	} else {
		if pingServer == vs.currentView.Primary {
			if vs.doSwitch() {
				vs.primaryAcked = false
			} else {
				vs.primaryAcked = true
				fmt.Printf("Been set as true:%v %v\n",pingServer, vs.currentView.Primary)
			}
		}
	}
	//update ttl
	if pingServer == vs.currentView.Primary {
		vs.primaryTTL = DeadPings
	} else if pingServer == vs.currentView.BackUp {
		vs.backupTTL = DeadPings
	} else {
		vs.idleServers[pingServer] = DeadPings
	}
	//返回当前view给server
	reply.View = *vs.currentView
	return nil
}

/**
和ping互斥
 */
func (vs *ViewServer) tick()  {
	vs.mu.Lock()
	defer vs.mu.Unlock()

	if vs.currentView == nil {
		return
	}

	//清理状态为down的backup
	//这里需要注意有一个潜在问题：如果tick阶段还没有清理掉状态为down的节点
	//那么，假如primay提升该节点为backup之后，primary down了。集群也就挂了
	for server, ttl := range vs.idleServers {
		if ttl <= 0 {
			delete(vs.idleServers, server)
		} else {
			vs.idleServers[server]--
		}
	}

	fmt.Printf("Current view before:%+v\n", vs.currentView)
	fmt.Printf("Has been acked?%v\n", vs.primaryAcked)
	if vs.primaryAcked && vs.updateAndSwitch() {
		vs.primaryAcked = false
	}
	fmt.Printf("Current view after:%+v\n", vs.currentView)

	/**
	集群只有一个节点，并且挂了
	 */
	if vs.currentView.Primary == "" {vs.primaryTTL = 0}

	/**
	没有idle提升为backup
	 */
	if vs.currentView.BackUp == "" {vs.backupTTL = 0}

	if vs.primaryTTL > 0 {vs.primaryTTL--}

	if vs.backupTTL >0 {vs.backupTTL--}
}

//
// server Get() RPC handler.
//
func (vs *ViewServer) Get(args *GetArgs, reply *GetReply) error {
	vs.mu.Lock()
	defer vs.mu.Unlock()

	if vs.currentView == nil {
		reply.View = *createView(0, "", "")
	} else {
		reply.View = *vs.currentView
	}
	return nil
}

//For testing
//
// tell the server to shut itself down.
// for testing.
// please don't change these two functions.
//
func (vs *ViewServer) Kill() {
	atomic.StoreInt32(&vs.dead, 1)
	vs.listener.Close()
}

//
// has this server been asked to shut down?
//
func (vs *ViewServer) isdead() bool {
	return atomic.LoadInt32(&vs.dead) != 0
}

// please don't change this function.
func (vs *ViewServer) GetRPCCount() int32 {
	return atomic.LoadInt32(&vs.rpcCount)
}

func StartServer(me string) *ViewServer {
	vs := new(ViewServer)
	vs.me = me
	vs.idleServers = make(map[string]int)

	rpcs := rpc.NewServer()
	rpcs.Register(vs)

	// client 和 server在同一台机器，用unix比较方便
	os.Remove(vs.me)
	listener,err := net.Listen("unix", vs.me)
	if err != nil {
		log.Fatal("listen error:%s", vs.me)
	}
	vs.listener = listener

	//listen at given port
	go func() {
		for vs.isdead() == false {
			conn,err := vs.listener.Accept()
			if err == nil && vs.isdead() == false {
				atomic.AddInt32(&vs.rpcCount, 1)
				go rpcs.ServeConn(conn)
			} else if err == nil {
				conn.Close()
			}

			if err != nil && vs.isdead() == false {
				fmt.Println("Listen failed:%v", err)
				vs.Kill()
			}
		}
	}()

	//tick
	go func() {
		for vs.isdead() == false {
			vs.tick()
			time.Sleep(PingInterval)
		}
	}()
	return vs
}

