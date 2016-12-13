package raft

import (
	"sync"
	"../labrpc"
	"math/rand"
	"time"
	"fmt"
)

const (
	LEADER  = iota
	FOLLOWER
	CANDIDATE
)

type ApplyMsg struct {
	Index       int
	Command     interface{}
	UseSnapshot bool   // ignore for lab2; only used in lab3
	Snapshot    []byte // ignore for lab2; only used in lab3
}

type Raft struct {
	mu sync.Mutex
	peers []*labrpc.ClientEnd
	persister *Persister
	me int

	currentTerm int
	votedFor int
	log []interface{}
	//for leaders
	nextIndex []int
	matchIndex []int

	role int //leader follower candidate

	resetTimeChan chan int
}

func (rf *Raft) persist() {
	// Your code here.
	// Example:
	// w := new(bytes.Buffer)
	// e := gob.NewEncoder(w)
	// e.Encode(rf.xxx)
	// e.Encode(rf.yyy)
	// data := w.Bytes()
	// rf.persister.SaveRaftState(data)
}

//
// restore previously persisted state.
//
func (rf *Raft) readPersist(data []byte) {
	// Your code here.
	// Example:
	// r := bytes.NewBuffer(data)
	// d := gob.NewDecoder(r)
	// d.Decode(&rf.xxx)
	// d.Decode(&rf.yyy)
}

/**
获取当前raft node的状态（crrent term和是否是leader）
 */
func (rf *Raft) GetState() (int, bool) {
	var term int
	var isLeader bool
	term = rf.currentTerm
	switch rf.role {
	case LEADER:
		isLeader = true
		break
	case FOLLOWER:
	case CANDIDATE:
		isLeader = false
		break
	}
	return  term, isLeader
}

/**
Request vote rpc arguments
 */
type RequestVoteArgs struct {
	Term int
	CandidateId int
	LastLogIndex int
	LastLogTerm int
}

type RequestVoteReply struct {
	Term int
	VoteGranted bool
}

func (rf *Raft) RequestVote(args *RequestVoteArgs, reply *RequestVoteReply)  {
	reply.Term = rf.currentTerm
	if args.Term < rf.currentTerm {
		reply.VoteGranted = false
	} else if rf.votedFor == -1 || rf.votedFor == args.CandidateId {
		rf.votedFor = args.CandidateId
		reply.VoteGranted = true
	}
}

func (rf *Raft) sendRequestVote(server int, args *RequestVoteArgs, reply *RequestVoteReply) bool {
	ok := rf.peers[server].Call("Raft.RequestVote", args, reply)
	return ok
}

func (rf *Raft) beginElection()  {
	rf.currentTerm++
	rf.role = CANDIDATE

	args := RequestVoteArgs{Term:rf.currentTerm, CandidateId:rf.me}
	votedServer := 1 // vote for self
	for index,_ := range rf.peers {
		if rf.me != index {
			reply := new(RequestVoteReply)
			ok := rf.sendRequestVote(index, args, reply)
			if ok && reply.VoteGranted {
				votedServer++
			}
		}
	}

	fmt.Println("VotedRes:", votedServer, "M:", len(rf.peers)/2)
	if votedServer > len(rf.peers)/2 {
		rf.role = LEADER
		//begin to send heartbeat
		rf.resetTimeChan <- 1
		rf.electInterval(1)
	} else {
		rf.role = FOLLOWER
		rf.resetTimeChan <- 1
		rf.electInterval(0)
	}
}

/**
Append entry rpc arguments
 */
type AppendEntriesArgs struct {
	Term int
	LeaderId int
	PrevLogIndex int
	PrevLogTerm int
	Entries []interface{}
	LeaderCommitTerm int
}

/**
Append entry rpc response
 */
type AppendEntriesReply struct {
	Term int
	Success bool
}

func (rf *Raft) AppendEntries(args *AppendEntriesArgs, reply *AppendEntriesReply) {
	rf.resetTimeChan <- 1
	//收到了appedn rpc，说明有leader
	rf.role = FOLLOWER
	rf.electInterval(0)

	if args.Term < rf.currentTerm {
		reply.Success = false
	} else {
		reply.Success = true
	}
	reply.Term = rf.currentTerm
}

func (rf *Raft) sendAppendEntries(server int, args *AppendEntriesArgs, reply *AppendEntriesReply) bool {
	ok := rf.peers[server].Call("Raft.AppendEntries", args, reply)
	return ok
}

func (rf *Raft) beginAppendEntries()  {
	for index, _ := range rf.peers {
		if index != rf.me {
			args := new(AppendEntriesArgs)
			args.Term = rf.currentTerm
			args.LeaderId = rf.me
			reply := new(AppendEntriesReply)
			ok := rf.sendAppendEntries(index, args, reply)
			if ok && reply.Success {
				//TODO
			} else {
				//TODO
			}
		}
	}
}

const (
	ELECTION = iota
	HEARTBEAT
)

func (rf *Raft) electInterval(action int)  {
	rand.Seed(time.Now().Unix() + int64(rf.me))
	millSeconds := time.Duration(rand.Intn(150) + 250)
	ticker := time.NewTicker(time.Millisecond * millSeconds)
	go func() {
		for  {
			select {
			case <-ticker.C:
				ticker.Stop()
				rand.Seed(time.Now().Unix() + int64(rf.me))
				millSeconds := time.Duration(rand.Intn(150) + 250)
				ticker = time.NewTicker(time.Millisecond * millSeconds)
				if action == ELECTION {

				} else if action == HEARTBEAT {
					rf.beginAppendEntries()
				}
			case <-rf.resetTimeChan:
				ticker.Stop()
				rand.Seed(time.Now().Unix() + int64(rf.me))
				millSeconds := time.Duration(rand.Intn(150) + 250)
				ticker = time.NewTicker(time.Millisecond * millSeconds)
				break
			}
		}
	} ()
}

func Make(peers []*labrpc.ClientEnd, me int, persister *Persister, applyCh chan ApplyMsg) *Raft {
	rf := &Raft{}
	rf.peers = peers
	rf.persister = persister
	rf.me = me
	rf.currentTerm = 0
	rf.role = FOLLOWER //跟随者
	rf.votedFor = -1
	rf.resetTimeChan = make(chan int)
	go rf.electInterval(0)
	// initialize from state persisted before a crash
	rf.readPersist(persister.ReadRaftState())

	return rf
}

//For testing
func (rf *Raft) Kill() {
	// Your code here, if desired.
}

func (rf *Raft) Start(command interface{}) (int, int, bool) {
	index := -1
	term := -1
	isLeader := true

	return index, term, isLeader
}