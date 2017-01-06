package mapreduce

import "fmt"

// schedule starts and waits for all tasks in the given phase (Map or Reduce).
func (mr *Master) schedule(phase jobPhase) {
	var ntasks int
	var nios int // number of inputs (for reduce) or outputs (for map)
	switch phase {
	case mapPhase:
		ntasks = len(mr.files)
		nios = mr.nReduce
	case reducePhase:
		ntasks = mr.nReduce
		nios = len(mr.files)
	}

	fmt.Printf("Schedule: %v %v tasks (%d I/Os)\n", ntasks, phase, nios)

	// All ntasks tasks have to be scheduled on workers, and only once all of
	// them have been completed successfully should the function return.
	// Remember that workers may fail, and that any given worker may finish
	// multiple tasks.
	doneChannel := make(chan bool)

	for i := 1; i<=ntasks ; i++ {
		go func(taskNumer int) {
			doTaskArgs := new(DoTaskArgs)
			doTaskArgs.File = mr.files[i]
			doTaskArgs.JobName = mr.jobName
			doTaskArgs.Phase = phase
			doTaskArgs.TaskNumber = i
			doTaskArgs.NumOtherPhase = nios
			var woker string
			ok := false
			reply := new(struct{})
			for !ok {
				woker = <- mr.registerChannel
				ok = call(woker, "Worker.DoTask", doTaskArgs, reply)
			}
			doneChannel <- true
			mr.registerChannel <- woker
		}(i)
	}

	for i := 1; i<=ntasks ; i++  {
		<- doneChannel
	}
	fmt.Printf("Schedule: %v phase done\n", phase)
}
