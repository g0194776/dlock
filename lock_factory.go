package dlock

import (
	"errors"
	"sync"

	"github.com/coreos/etcd/clientv3"
)

var (
	lockObj *sync.Mutex
	lockMap = make(map[string]DistributedLocker)
)

type DistributedLocker interface {
	TryGetLock() error
	Close()
}

//This is a simple object which holds by the real lock.
//Add more contextual information if needed.
type DistributedLockStub struct {
	Owner    string
	isMaster *int32
	msgChan  chan string
	lease    *clientv3.LeaseGrantResponse
}

type DistributedLockOptions struct {
	etcdClient  *clientv3.Client
	Key         string
	ETCDAddress string
	TTL         int
	//It'll be trigger when get lock.
	HoldingLockFunc func()
	//It'll be trigger when lost lock.
	LosingLockFunc func()
}

func init() {
	if lockObj == nil {
		lockObj = &sync.Mutex{}
	}
}

//Immediately create a new distributed lock instance when there is no any instance with the same Key being created before.
func NewDistributedLock(option DistributedLockOptions) (DistributedLocker, error) {
	if option.Key == "" {
		return nil, errors.New("option.Key must be set.")
	}
	if option.HoldingLockFunc == nil {
		return nil, errors.New("option.HoldingLockFunc is required for receiving notification.")
	}
	if option.LosingLockFunc == nil {
		return nil, errors.New("option.LosingLockFunc is required for receiving notification.")
	}
	//set default value.
	if option.TTL == 0 {
		option.TTL = 5
	}
	lockObj.Lock()
	defer lockObj.Unlock()
	var isOK bool
	var l DistributedLocker
	if l, isOK = lockMap[option.Key]; isOK {
		return l, nil
	}
	l = &distributedLock{options: &option}
	lockMap[option.Key] = l
	return l, nil
}
