[mit-6.824 lab1文档](https://pdos.csail.mit.edu/6.824/labs/lab-1.html)

这部分是实现和理解mapreduce论文，实现简单的mapreduce框架

## 主要设计点

1. 先执行完map再执行reduce

2. 没有在map之后对同一个key做merge操作

## Part1
实现doMap和doReduce函数

根据论文中的流程图：

1. master调度worker之后（调度是part3要完成的），需要worker完成相应工作

2. doMap是负责执行实际的map任务，主要输入是jobname，filepath，map函数（part2需要实现的）

3. doReduce负责执行reduce任务

## Part2

实现mapF 和 reduceF （word count）

这里我理解的是，客户端自定义这两个函数，通过反射代理实现调用（java）

## Part3 Part4

实现调度算法：

1. 文件已经在master中被分成多个部分，所以这里只需要选择register的
   worker来执行部分文件map
   
2. 通过channel实现，由于需要复用woker资源，所以map之后返回woker到registerchannel，供master调度

3. go routine实现并发执行map或者reduce

4. 容错？schedule函数里面，是一个for循环，直到任务执行成功（所以会循环调度，直到找到可以成功执行的worker）

5. 操作幂等性：所以上一步，重复执行没关系（因为可能是rpc错误也可能是worker fail了，所以可能会重复执行）