package pbservice

import (
	"net/rpc"
	"fmt"
)

const (
	OK              = "OK"
	ErrNoKey        = "ErrNoKey"
	ErrWrongServer  = "ErrWrongServer"
	ErrUninitServer = "ErrUninitServer"
)

/**
方法名（支持的操作名称）
 */
const (
	Get       = "Get"
	Put       = "Put"
	Append    = "Append"
)

type Err string

type InitStateArgs struct {
	State map[string]string
}

type InitStateReply struct {
	Err   Err
}

type PutAppendArgs struct {
	Key   string
	Value string

	OpID     int64
	Method   string //表示是put还是append

	// Field names must start with capital letters,
	// otherwise RPC will break.
}

type PutAppendReply struct {
	Err Err
}

type TransferStateArgs struct {
	Target string
}

type TransferStateReply struct {
}

type GetArgs struct {
	Key string
	OpId int64 //唯一标志客户端操作
}

type GetReply struct {
	Err Err
	Value string
}

func copyGetReply(src *GetReply, dst *GetReply)  {
	dst.Err = src.Err
	dst.Value = src.Value
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