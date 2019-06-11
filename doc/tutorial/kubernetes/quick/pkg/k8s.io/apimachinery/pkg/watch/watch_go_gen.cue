// Code generated by cue get go. DO NOT EDIT.

//cue:generate cue get go k8s.io/apimachinery/pkg/watch

package watch

import (
	"k8s.io/apimachinery/pkg/runtime"
)

// Interface can be implemented by anything that knows how to watch and report changes.
Interface: _

// EventType defines the possible types of events.
EventType: string // enumEventType

enumEventType:
	Added |
	Modified |
	Deleted |
	Bookmark |
	Error

Added:           EventType & "ADDED"
Modified:        EventType & "MODIFIED"
Deleted:         EventType & "DELETED"
Bookmark:        EventType & "BOOKMARK"
Error:           EventType & "ERROR"
DefaultChanSize: int32 & 100

// Event represents a single event to a watched resource.
// +k8s:deepcopy-gen=true
Event: {
	Type: EventType

	// Object is:
	//  * If Type is Added or Modified: the new state of the object.
	//  * If Type is Deleted: the state of the object immediately before deletion.
	//  * If Type is Bookmark: the object (instance of a type being watched) where
	//    only ResourceVersion field is set. On successful restart of watch from a
	//    bookmark resourceVersion, client is guaranteed to not get repeat event
	//    nor miss any events.
	//  * If Type is Error: *api.Status is recommended; other types may make sense
	//    depending on context.
	Object: runtime.Object
}

// FakeWatcher lets you test anything that consumes a watch.Interface; threadsafe.
FakeWatcher: {
	Stopped: bool
}

// RaceFreeFakeWatcher lets you test anything that consumes a watch.Interface; threadsafe.
RaceFreeFakeWatcher: {
	Stopped: bool
}
