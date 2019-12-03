package server

// BroadcastSessionsSelector selects the next BroadcastSession to use
type BroadcastSessionsSelector interface {
	Add(sessions []*BroadcastSession)
	Complete(sess *BroadcastSession)
	Select() *BroadcastSession
	Size() int
	Clear()
}

// LIFOSelector selects the next BroadcastSession in LIFO order
type LIFOSelector []*BroadcastSession

// Add adds the sessions to the front of the selector's list
func (s *LIFOSelector) Add(sessions []*BroadcastSession) {
	*s = append(sessions, *s...)
}

// Complete adds the session to the end of the selector's list
func (s *LIFOSelector) Complete(sess *BroadcastSession) {
	*s = append(*s, sess)
}

// Select returns the last session in the selector's list
func (s *LIFOSelector) Select() *BroadcastSession {
	sessList := *s
	last := len(sessList) - 1
	sess, sessions := sessList[last], sessList[:last]
	*s = sessions
	return sess
}

// Size returns the number of sessions stored by the selector
func (s *LIFOSelector) Size() int {
	return len(*s)
}

// Clear resets the selector's state
func (s *LIFOSelector) Clear() {
	*s = nil
}
