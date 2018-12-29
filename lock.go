package distributed_lock

type distributedLock struct {
	options *DistributedLockOptions
	stub    *DistributedLockStub
}

func (*distributedLock) GetLock() (*DistributedLockStub, error) {
	panic("implement me")
}

func (*distributedLock) ReleaseLock() error {
	panic("implement me")
}
