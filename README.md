# distributed-lock

## Preface
After considered many of implementations of distributed lock with Golang, I prefered like to write it by myself. 

This is an ETCD-based implementation and used v3 protocol for performance enhancement. Simple & Easy, enjoy it!

## How to test?

#### 1. Start an ETCD instance using Docker container
```shell
docker run -d --name etcd-server \
--publish 2379:2379 \
--publish 2380:2380 \
--env ALLOW_NONE_AUTHENTICATION=yes \
--env ETCD_ADVERTISE_CLIENT_URLS=http://etcd-server:2379 \
bitnami/etcd:latest
```

#### 2. Code Sample
```go
dl, err := NewDistributedLock(DistributedLockOptions{
    Key:         "/KEY-SPEC-100",
    ETCDAddress: "http://127.0.0.1:2379", //accessible ETCD instance.
    TTL:         5,
    //hooking function to receive an event while acquiring the lock.
    HoldingLockFunc: func(locker DistributedLocker, stub DistributedLockStub) {
        fmt.Println("You are master now...!")

    },
    //hooking function to receive an event while losing the lock.
    LosingLockFunc: func(locker DistributedLocker, stub DistributedLockStub) {
        fmt.Println("You've lost master role, waiting...")
    },
})
if err != nil {
	panic(err)
}

dl.TryGetLock()
```