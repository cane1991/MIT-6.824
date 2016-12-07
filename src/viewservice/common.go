package viewservice

import "time"

/**
表示一个 view
 */
type View struct {

	Viewnum uint
	Primary string
	BackUp string
}

/**
集群节点ping VS的周期
 */
const PingInterval  = time.Second * 100

/**

 */
const DeadPings  = 5

type PingArgs struct {
	Me string //server的信息
	ViewNum uint //server当前acknowledgement的view序号
}

type PingReply struct {
	View View
}

/**
For p/b and testing
 */
type GetArgs struct {

}

type GetReply struct {
	View View
}




