package dlock

import (
	"context"
	"fmt"
	"github.com/coreos/etcd/clientv3"
	"github.com/pkg/errors"
	"os"
	"sync/atomic"
	"time"
)

type distributedLock struct {
	options *DistributedLockOptions
	stub    *DistributedLockStub
}

func (dl *distributedLock) TryGetLock() error {
	var err error
	if dl.options.etcdClient == nil {
		dl.options.etcdClient, err = clientv3.New(clientv3.Config{
			Endpoints:   []string{dl.options.ETCDAddress},
			DialTimeout: 5 * time.Second,
		})
	}
	if dl.stub == nil {
		var x int32
		host, _ := os.Hostname()
		dl.stub = &DistributedLockStub{isMaster: &x, Owner: host}
	}
	if err != nil {
		return errors.WithStack(fmt.Errorf("Failed to initialize ETCD client: %s", err.Error()))
	}
	grantedLease, err := dl.options.etcdClient.Grant(context.TODO(), int64(dl.options.TTL))
	if err != nil {
		return errors.WithStack(fmt.Errorf("Failed to grant lease: %s", err.Error()))
	}
	//keep lease alive forever until the process closed.
	dl.options.etcdClient.KeepAlive(context.TODO(), grantedLease.ID)
	cmp := clientv3.Compare(clientv3.CreateRevision(dl.options.Key), "=", 0)
	put := clientv3.OpPut(dl.options.Key, dl.stub.Owner, clientv3.WithLease(grantedLease.ID))
	get := clientv3.OpGet(dl.options.Key)
	getOwner := clientv3.OpGet(dl.options.Key /*"/master-role-spec"*/, clientv3.WithFirstCreate()...)
	rsp, err := dl.options.etcdClient.Txn(dl.options.etcdClient.Ctx()).If(cmp).Then(put, getOwner).Else(get, getOwner).Commit()
	if err != nil {
		return errors.WithStack(err)
	}
	if dl.isLockMaster(rsp) {
		if atomic.CompareAndSwapInt32(dl.stub.isMaster, 0, 1) {
			go dl.options.HoldingLockFunc(dl, DistributedLockStub{})
		}
		return nil
	}
	return nil
}

func (dl *distributedLock) isLockMaster(rsp *clientv3.TxnResponse) bool {
	if rsp.Succeeded {
		return true
	}
	v := string(rsp.Responses[0].GetResponseRange().Kvs[0].Value)
	revision := rsp.Responses[0].GetResponseRange().Kvs[0].CreateRevision
	ownerKey := rsp.Responses[1].GetResponseRange().Kvs
	host, _ := os.Hostname()
	if (len(ownerKey) == 0 || ownerKey[0].CreateRevision == revision) && v == host {
		return true
	}
	return false
}
