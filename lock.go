package dlock

import (
	"context"
	"fmt"
	"github.com/coreos/etcd/clientv3"
	"github.com/pkg/errors"
	"github.com/satori/go.uuid"
	log "github.com/sirupsen/logrus"
	"os"
	"sync/atomic"
	"time"
)

type distributedLock struct {
	options *DistributedLockOptions
	stub    *DistributedLockStub
}

func (dl *distributedLock) TryGetLock() error {
	err := dl.doInitialize()
	if err != nil {
		return err
	}
	rsp, err := dl.getKeyInformation()
	if err != nil {
		return errors.WithStack(fmt.Errorf("Failed to execute ETCD transaction: %s", err.Error()))
	}
	if dl.isLockMaster(rsp) {
		log.Info("You are holding the lock!")
		if atomic.CompareAndSwapInt32(dl.stub.isMaster, 0, 1) {
			go dl.options.HoldingLockFunc()
		}
		return nil
	} else {
		log.Info("You are not holding the lock, waiting...")
	}
	return nil
}

func (dl *distributedLock) doInitialize() error {
	var err error
	if dl.options.etcdClient == nil {
		dl.options.etcdClient, err = clientv3.New(clientv3.Config{
			Endpoints:   []string{dl.options.ETCDAddress},
			DialTimeout: 5 * time.Second,
		})
	}
	if err != nil {
		return errors.WithStack(fmt.Errorf("Failed to initialize ETCD client: %s", err.Error()))
	}
	if dl.stub == nil {
		var x int32
		id := uuid.Must(uuid.NewV4())
		dl.stub = &DistributedLockStub{isMaster: &x, Owner: id.String(), msgChan: make(chan string)}
		dl.stub.lease, err = dl.options.etcdClient.Grant(context.TODO(), int64(dl.options.TTL))
		if err != nil {
			return errors.WithStack(fmt.Errorf("Failed to grant lease: %s", err.Error()))
		}
		//keep lease alive forever until the process closed.
		dl.options.etcdClient.KeepAlive(context.TODO(), dl.stub.lease.ID)
		//asynchronously run new go-routines for monitoring changes.
		go dl.monitorLock()
		go dl.doSyncState()
	}
	return nil
}

func (dl *distributedLock) getKeyInformation() (*clientv3.TxnResponse, error) {
	cmp := clientv3.Compare(clientv3.CreateRevision(dl.options.Key), "=", 0)
	put := clientv3.OpPut(dl.options.Key, dl.stub.Owner, clientv3.WithLease(dl.stub.lease.ID))
	get := clientv3.OpGet(dl.options.Key)
	getOwner := clientv3.OpGet(dl.options.Key /*"/master-role-spec"*/, clientv3.WithFirstCreate()...)
	return dl.options.etcdClient.Txn(dl.options.etcdClient.Ctx()).If(cmp).Then(put, getOwner).Else(get, getOwner).Commit()
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

func (dl *distributedLock) monitorLock() {
	watcher := clientv3.NewWatcher(dl.options.etcdClient)
	defer watcher.Close()
	wc := watcher.Watch(context.Background(), dl.options.Key)
	for {
		wr, isOK := <-wc
		if !isOK {
			break
		}
		if wr.Events != nil && len(wr.Events) > 0 {
			for i := 0; i < len(wr.Events); i++ {
				if wr.Events[i].Type == clientv3.EventTypeDelete {
					dl.stub.msgChan <- "deleted"
				}
			}
		}
	}
}

func (dl *distributedLock) doSyncState() {
	var msg string
	var isOK bool
	for {
		msg, isOK = <-dl.stub.msgChan
		if !isOK {
			break
		}
		//ignored other events except DELETED.
		if msg == "deleted" {
			if atomic.CompareAndSwapInt32(dl.stub.isMaster, 1, 0) {
				go dl.options.LosingLockFunc()
			}
			//repeatedly get the lock.
			dl.TryGetLock()
		}
	}
}

func (dl *distributedLock) Close() {
	defer func() {
		recover()
	}()
	if dl.stub != nil {
		if dl.stub.msgChan != nil {
			close(dl.stub.msgChan)
		}
		dl.stub = nil
	}
}
