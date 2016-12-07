## Part A: The Viewservice

[mit-6.824 lab2 文档](http://nil.csail.mit.edu/6.824/2015/labs/lab-2.html)

这个版本实现的是View Service

(仅供学习使用)
### 系统角色
1. View Server (管理集群的节点，控制primary的切换，保证集群只有一个primary，提升idle为backup，提升backup为primary)
2. Primary (store server的集群主节点)
3. BackUp (可以理解为slave，需要sync -> Primary，最有一个)
4. Idle server (只是和VS ping,合适的时候提升为backup)

#### 关于角色切换的说明
vs的存在是为了切换primary，保证primary 挂机之后选择新的primary,有以下情况
##### 1. primary crash然后reboot，ping VS
    > (1) primary为当前view的情况，已经被ack了（就是primary在down之前，已经知道自己就是当前view的primary）
  
    >  这个时候，假如有backup，就promote为新的primary，返回给server;同时，选择idle队列的一个server为新的backup
    
    >  没有backup，那么集群就挂机，由于idle不为空的话一定为保证有backup，所有假如没有backup，说明集群只有一个节点，不正常服务也是对的
  
    > (2) primary还没有ack当前的view，那么集群必须等到这个primary reboot之后才会正常服务
  
##### 2. primary ping
    > 更新ttl

##### 3. 非backup ping
    > 放到idle队列，更新ttl（VS后台需要提供任务，定时清理down掉的idle server）
    
##### 4. backup ping
    > 更新ttl

### 系统功能
    > *内存*kv数据存储

### view
    > view是集群的当前视图，表明集群哪个节点是Primary,哪个是BackUp

### 节点状态
1. active
2. recovering
3. down
    > 如果down -> active，需要给primary汇报这个信息（方法使用过特殊的ping参数）

### tips
    > 1. 集群每个节点需要在固定时间间隔之内ping VS(View Server的简称)

    > 2. 只有三种情况可以更新view，也就是view的number递增(详细参考上面的链接)

    > 3. 如果primary不能返回ping acknowledges给VS，集群将无法自动恢复
    
### 测试
    > cd到viewsercice目录，执行*go test*
