/**
KV-Store:master slave架构
 */
package pbservice

import (
	"sync"
	"net"
	"../viewservice"
	"time"
	"fmt"
	"syscall"
	"net/rpc"
	"os"
	"log"
	"sync/atomic"
	"math/rand"
)

const CacheOperationTime  = int(10 * time.Second)

type PBServer struct {
	dead       int32 // for testing
	unreliable int32 // for testing

	mu sync.Mutex
	listener net.Listener

	//缓存的view
	view *viewservice.View

	//viewservice的client
	viewserviceClient *viewservice.Clerk

	//是否初始化
	init bool

	//服务器的地址，ping的时候发送给viewservice
	me string

	//kv store
	kvStore map[string]string

	//对于重试的客户端，避免多次执行同样的操作
	operationFilter map[int64]int
	cachedReply map[int64]interface{}
}

func (pb *PBServer) isInited() bool {
	return pb.init
}

func (pb *PBServer) InitState(args *InitStateArgs, reply *InitStateReply) error {
	pb.mu.Lock()
	defer pb.mu.Unlock()

	if !pb.isInited() {
		pb.init = true
		pb.kvStore = args.State //back up同步
	}
	reply.Err = OK
	return nil
}

func (pb *PBServer) transferState(server string) bool {
	if server != pb.view.BackUp {
		return false
	}

	args := new(InitStateArgs)
	args.State = pb.kvStore
	var reply InitStateReply
	call(server, "PBServer.InitState", args, &reply)
	if reply.Err == OK {
		return true
	}
	return false
}

func (pb *PBServer) filterDuplicateOperation(opId int64, method string, reply interface{}) bool {
	_, ok := pb.operationFilter[opId]
	fmt.Printf("Check for Opid:%v %s",opId, method)
	if !ok {
		return false
	}

	rp, ok := pb.cachedReply[opId]
	if !ok {
		return false
	}

	if method == Get {
		reply, ok1 := reply.(*GetReply)
		cache, ok2 := rp.(*GetReply)
		if ok1 && ok2 {
			copyGetReply(cache, reply)
			return true
		}
	}
	return false
}

func (pb *PBServer) cacheOperation(opId int64, reply interface{})  {
	pb.operationFilter[opId] = CacheOperationTime
	pb.cachedReply[opId] = reply
}

func (pb *PBServer) doGet(key string, reply *GetReply)  {
	value, ok := pb.kvStore[key]
	if !ok {
		reply.Err = ErrNoKey
		return
	}
	reply.Value = value
}

/**
backup升级为primary之后，为了避免重复请求所以get请求也转发backup
 */
func (pb *PBServer) BackupGet(args *GetArgs, reply *GetReply) error {
	pb.mu.Lock()
	defer pb.mu.Unlock()
	fmt.Printf("RPC : BackupGet : server %s\n", pb.me)

	if pb.me != pb.view.BackUp {
		reply.Err = ErrWrongServer
		return nil
	}

	// the request is from primary and we are backup

	if !pb.isInited() {
		reply.Err = ErrUninitServer
		return nil
	}

	ok := pb.filterDuplicateOperation(args.OpId, Get, reply)
	if ok {
		fmt.Printf("Duplicate : reply.Err : %s\n", reply.Err)
		return nil
	}

	pb.doGet(args.Key, reply)

	pb.cacheOperation(args.OpId, reply)

	return nil
}

func (pb *PBServer) Get(getArgs *GetArgs, getReply *GetReply) error {
	//串行操作
	pb.mu.Lock()
	defer pb.mu.Unlock()

	/**
	对于网络延时的情况，没办法处理
	 */
	if pb.me != pb.view.Primary {
		getReply.Err = ErrWrongServer
		return nil
	}

	ok := pb.filterDuplicateOperation(getArgs.OpId, Get, getReply)
	if ok {
		fmt.Printf("Duplicate get operation:%v %v",getArgs.OpId, getReply.Err)
		return nil
	}

	if pb.view.BackUp != "" {
		ok := call(pb.view.BackUp, "PBServer.BackupGet", getArgs, getReply)
		if ok {
			if getReply.Err == ErrUninitServer {
				pb.transferState(pb.view.BackUp)
			}
		} else {
			getReply.Err = ErrWrongServer
			return nil
		}
	}
	
	pb.doGet(getArgs.Key, getReply)
	pb.cacheOperation(getArgs.OpId, getReply)
	return nil
}

func (pb *PBServer) doPutAppend(args *PutAppendArgs, reply *PutAppendReply) error {
	fmt.Printf("--- op : %s, key : %s, value : %s\n", args.Method, args.Key, args.Value)

	key, value := args.Key, args.Value
	method := args.Method
	if method == Put {
		pb.kvStore[key] = value
	} else if method == Append {
		pb.kvStore[key] += value
	}
	reply.Err = OK

	return nil
}

func (pb *PBServer) PutAppend(args *PutAppendArgs, reply *PutAppendReply) error {
	pb.mu.Lock()
	defer pb.mu.Unlock()
	fmt.Printf("RPC : PutAppend : server %s\n", pb.me)

	if pb.me != pb.view.Primary {
		reply.Err = ErrWrongServer
		return nil
	}

	ok := pb.filterDuplicateOperation(args.OpID, args.Method, reply)
	if ok {
		fmt.Printf("Duplicate : reply.Err : %s\n", reply.Err)
		return nil
	}

	xferafter :=  false
	if pb.view.BackUp != "" {
		fmt.Printf("forward %s to backup %s\n", args.Method, pb.view.BackUp);

		tries := 1 // tring again doesn't help in many cases
		for tries > 0 {
			ok := call(pb.view.BackUp, "PBServer.BackupPutAppend", args, reply)
			if ok {
				if reply.Err == ErrWrongServer {
					return nil
				}
				if reply.Err == ErrUninitServer {
					xferafter = true
				}
				break
			}
			fmt.Printf("retry RPC BackupPutAppend %d ...\n", tries)
			tries--
		}
		if tries == 0 {
			reply.Err = ErrWrongServer
			return nil
		}
	}

	pb.doPutAppend(args, reply)
	pb.cacheOperation(args.OpID, reply)

	if xferafter {
		pb.transferState(pb.view.BackUp)
	}

	return nil
}

func (pb *PBServer) BackupPutAppend(args *PutAppendArgs, reply *PutAppendReply) error {
	pb.mu.Lock()
	defer pb.mu.Unlock()
	fmt.Printf("RPC : BackupPutAppend : server %s\n", pb.me)

	if pb.me != pb.view.BackUp {
		reply.Err = ErrWrongServer
		return nil
	}

	if !pb.isInited() {
		reply.Err = ErrUninitServer
		return nil
	}

	ok := pb.filterDuplicateOperation(args.OpID, args.Method, reply)
	if ok {
		fmt.Printf("Duplicate : reply.Err : %s\n", reply.Err)
		return nil
	}

	pb.doPutAppend(args, reply)
	pb.cacheOperation(args.OpID, reply)

	return nil
}

func (pb *PBServer) TransferState(
args *TransferStateArgs, reply *TransferStateReply) error {
	pb.mu.Lock()
	defer pb.mu.Unlock()

	pb.transferState(args.Target)

	return nil
}

func (pb *PBServer) transferStateByRpc(server string) {
	args := &TransferStateArgs{}
	args.Target = pb.me
	var reply TransferStateReply
	go call(server, "PBServer.TransferState", args, &reply)
}

func (pb *PBServer) pingViewServer() *viewservice.View {
	viewNum := uint(0)
	if pb.view != nil {
		viewNum = pb.view.Viewnum
	}
	view, err := pb.viewserviceClient.Ping(viewNum)
	if err != nil {
		return nil
	}
	return &view
}

func (pb *PBServer) cleanUpCachedOperations()  {
	for opId, ttl := range pb.operationFilter{
		if ttl <= 0 {
			delete(pb.operationFilter, opId)
			delete(pb.cachedReply, opId)
		} else {
			pb.operationFilter[opId]--
		}
	}
}

/**
1.ping viewserver
2.cleanup cached filter operations
 */
func (pb *PBServer) tick()  {
	pb.mu.Lock()
	defer pb.mu.Unlock()

	view := pb.pingViewServer()
	if view != nil {
		if !pb.isInited() {
			if pb.me == view.BackUp {
				pb.transferStateByRpc(pb.me)
			}	
		}
		pb.view = view
	}

	pb.cleanUpCachedOperations()
}

// tell the server to shut itself down.
// please do not change these two functions.
func (pb *PBServer) kill() {
	atomic.StoreInt32(&pb.dead, 1)
	pb.listener.Close()
}

func (pb *PBServer) isdead() bool {
	return atomic.LoadInt32(&pb.dead) != 0
}

// please do not change these two functions.
func (pb *PBServer) setunreliable(what bool) {
	if what {
		atomic.StoreInt32(&pb.unreliable, 1)
	} else {
		atomic.StoreInt32(&pb.unreliable, 0)
	}
}

func (pb *PBServer) isunreliable() bool {
	return atomic.LoadInt32(&pb.unreliable) != 0
}

func StartServer(vshost string, me string) *PBServer {
	pb := new(PBServer)
	pb.me = me
	pb.viewserviceClient = viewservice.MakeClerk(me, vshost)
	// Your pb.* initializations here.
	pb.kvStore = make(map[string]string)
	pb.operationFilter = make(map[int64]int)
	pb.cachedReply = make(map[int64]interface{})

	rpcs := rpc.NewServer()
	rpcs.Register(pb)

	os.Remove(pb.me)
	l, e := net.Listen("unix", pb.me)
	if e != nil {
		log.Fatal("listen error: ", e)
	}
	pb.listener = l

	// please do not change any of the following code,
	// or do anything to subvert it.

	go func() {
		for pb.isdead() == false {
			conn, err := pb.listener.Accept()
			if err == nil && pb.isdead() == false {
				if pb.isunreliable() && (rand.Int63()%1000) < 100 {
					// discard the request.
					conn.Close()
				} else if pb.isunreliable() && (rand.Int63()%1000) < 200 {
					// process the request but force discard of reply.
					c1 := conn.(*net.UnixConn)
					f, _ := c1.File()
					err := syscall.Shutdown(int(f.Fd()), syscall.SHUT_WR)
					if err != nil {
						fmt.Printf("shutdown: %v\n", err)
					}
					go rpcs.ServeConn(conn)
				} else {
					go rpcs.ServeConn(conn)
				}
			} else if err == nil {
				conn.Close()
			}
			if err != nil && pb.isdead() == false {
				fmt.Printf("PBServer(%v) accept: %v\n", me, err.Error())
				pb.kill()
			}
		}
	}()

	go func() {
		for pb.isdead() == false {
			pb.tick()
			time.Sleep(viewservice.PingInterval)
		}
	}()

	return pb
}
