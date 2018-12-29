package distributed_lock

import (
	"errors"
	"sync"
)

var (
	lockObj *sync.Mutex
	lockMap = make(map[string]DistributedLocker)
)

type DistributedLocker interface {
	GetLock() (*DistributedLockStub, error)
	ReleaseLock() error
}

//This is a simple object which holds by the real lock.
//Add more contextual information if needed.
type DistributedLockStub struct {
	Owner string
}

type DistributedLockOptions struct {
	Key        string
	ETCDKeyTTL int
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
