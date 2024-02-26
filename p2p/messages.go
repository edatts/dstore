package p2p

// A message contains arbitrary data that is sent over
// a transport between two nodes.
type Message struct {
	Content []byte
}
