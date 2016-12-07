package viewservice

import (
	"net/rpc"
	"fmt"
)

type Clerk struct {
	me string
	viewServer string
}

func MakeClerk(me string, viewServer string) *Clerk {
	clerk := new(Clerk)
	clerk.me = me
	clerk.viewServer = viewServer
	return clerk
}

func call(server string, rpcName string, args interface{}, reply interface{}) bool {
	conn,err := rpc.Dial("unix", server)
	if err != nil {
		return false
	}
	defer conn.Close()
	err = conn.Call(rpcName, args, reply)
	if err == nil {
		return true
	}
	fmt.Println("Rpc call error:%v",err)
	return false;
}

func (clerk *Clerk) Ping(viewNum uint) (View, error) {
	args := new(PingArgs)
	args.Me = clerk.me
	args.ViewNum = viewNum
	var reply PingReply

	ok := call(clerk.viewServer, "ViewServer.Ping", &args, &reply)
	if ok {
		return reply.View, nil
	}
	return View{}, fmt.Errorf("Ping %v failed", viewNum)
}

func (clerk *Clerk) Get() (View, bool) {
	args := &GetArgs{}
	var reply GetReply
	ok := call(clerk.viewServer, "ViewServer.Get", args, &reply)
	if ok == false {
		return View{}, false
	}
	return reply.View, true
}

func (clerk *Clerk) Primary() string {
	v, ok := clerk.Get()
	if ok {
		return v.Primary
	}
	return ""
}