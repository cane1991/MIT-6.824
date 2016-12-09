package pbservice

import (
	"time"
	"math/big"
	"../viewservice"
	"crypto/rand"
)

type Clerk struct {
	vs *viewservice.Clerk
	// Your declarations here
	view  *viewservice.View
}

// this may come in handy.
func nrand() int64 {
	max := big.NewInt(int64(1) << 62)
	bigx, _ := rand.Int(rand.Reader, max)
	x := bigx.Int64()
	return x
}

func MakeClerk(vshost string, me string) *Clerk {
	ck := new(Clerk)
	ck.vs = viewservice.MakeClerk(me, vshost)
	// Your ck.* initializations here
	return ck
}

func (ck *Clerk) getView(userpc bool) *viewservice.View {
	if ck.view == nil || userpc {
		v, ok := ck.vs.Get()
		if ok {
			ck.view = &v
		} else { ck.view = nil }
	}
	return ck.view
}

func (ck *Clerk) getPrimary(userpc bool) string {
	v := ck.getView(userpc)
	if v != nil { return v.Primary }
	return ""
}

/**
  * Note :
  *   We can assume that each clerk has only one outstanding
  *   Put or Get.
  */

//
// fetch a key's value from the current primary;
// if they key has never been set, return "".
// Get() must keep trying until it either the
// primary replies with the value or the primary
// says the key doesn't exist (has never been Put().
//
func (ck *Clerk) Get(key string) string {
	args := &GetArgs{Key:key, OpId:nrand()}
	var reply GetReply

	userpc := false
	for {
		primary := ck.getPrimary(userpc)

		call(primary, "PBServer.Get", args, &reply)

		if reply.Err == OK || reply.Err == ErrNoKey {
			return reply.Value
		} else {
			userpc = true
		}

		time.Sleep(viewservice.PingInterval)
	}
	return "???"
}

//
// send a Put or Append RPC
//
func (ck *Clerk) PutAppend(key string, value string, op string) {
	args := &PutAppendArgs{Key:key, Value:value, Method:op, OpID:nrand()}
	var reply PutAppendReply

	userpc := false
	for {
		primary := ck.getPrimary(userpc)

		call(primary, "PBServer.PutAppend", args, &reply)

		if reply.Err == OK {
			break
		} else {
			userpc = true
		}

		time.Sleep(viewservice.PingInterval)
	}
}

//
// tell the primary to update key's value.
// must keep trying until it succeeds.
//
func (ck *Clerk) Put(key string, value string) {
	ck.PutAppend(key, value, Put)
}

//
// tell the primary to append to key's value.
// must keep trying until it succeeds.
//
func (ck *Clerk) Append(key string, value string) {
	ck.PutAppend(key, value, Append)
}
