package core

import "sync"

// Operation is how far the work running in one server directory has got.
type Operation struct {
	Label string
	Done  int64
	Total int64

	// Finished says the work is over and this is its final state. The reader
	// clears the entry once it has drawn it: an operation that deleted its own
	// progress the moment it ended would leave the last thing drawn stuck at
	// whatever the reader happened to sample before that.
	Finished bool
}

// progress records the operation running in each server directory.
//
// An install reports its progress from a background command, while the console
// that draws it lives in another model entirely; passing a channel between them
// would mean every panel that can start an operation holding a reference to the
// console. Instead the operation leaves its progress here, keyed by the folder
// it is working in, and the console reads whatever is there when it next
// refreshes. One entry per server, so two servers installing at once do not
// draw over each other.
var progress sync.Map

// Report records how far an operation in a directory has got.
func Report(dir, label string, done, total int64) {
	progress.Store(dir, Operation{Label: label, Done: done, Total: total})
}

// Progress returns the operation running in a directory, if there is one.
func Progress(dir string) (Operation, bool) {
	value, ok := progress.Load(dir)
	if !ok {
		return Operation{}, false
	}
	return value.(Operation), true
}

// Finish marks the operation in a directory as over, leaving its final state
// for whoever is drawing it.
func Finish(dir string) {
	operation, _ := Progress(dir)
	operation.Finished = true

	progress.Store(dir, operation)
}

// ClearProgress forgets the operation in a directory. It is called by the
// reader, once it has drawn the final state.
func ClearProgress(dir string) {
	progress.Delete(dir)
}
