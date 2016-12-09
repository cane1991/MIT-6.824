## Part B: The PBService

[mit-6.824 lab2 文档](http://nil.csail.mit.edu/6.824/2015/labs/lab-2.html)

这个部分实现的是**内存**的kv存储服务

## PBserver
server主要有3种角色:

1. primary:接收所有的**Get**,**Put**,**Append**操作
    
2. backup:同步primary，必要的时候切换为primary（由ViewService操作）
    
3. idle:必要时切换为backup（由ViewService操作）
    
## Client
client通过rpc访问PBserver:

    1.每次cache一个view，然后直接访问primary，避免viewservice节点负载过高
    
    2.如果缓存的primary是down的状态，那么就rp访问viewservice获取最新的view
    
## System Design
### IO
使用的是串行io(生产环境中使用的存储肯定需要重新设计的)

## 测试
cd到pbservice 执行 **go test**
    
## 缺陷
1. viewservice 单点问题

2. bio模式

