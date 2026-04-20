package registry

func NewSourceClient() SourceClient {
	return NewContainersImageClient()
}
