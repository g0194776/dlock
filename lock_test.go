package dlock

import (
	"fmt"
	"github.com/stretchr/testify/assert"
	"testing"
	"time"
)

func Test_SpecifiedKeyNotExists(t *testing.T) {
	dl, err := NewDistributedLock(DistributedLockOptions{
		Key:         "/KEY-SPEC-100",
		ETCDAddress: "http://127.0.0.1:2379",
		TTL:         5,
		HoldingLockFunc: func(locker DistributedLocker, stub DistributedLockStub) {
			fmt.Println("You are master now...!")
		},
		LosingLockFunc: func(locker DistributedLocker, stub DistributedLockStub) {
			fmt.Println("You've lost master role, waiting...")
		},
	})
	assert.Nil(t, err)
	assert.Nil(t, dl.TryGetLock())
	fmt.Printf("%#v\n", dl)
	time.Sleep(30 * time.Second)
}
