package activity

/*
#cgo LDFLAGS: -framework ApplicationServices
#include <ApplicationServices/ApplicationServices.h>

static uint32_t pc_counter(CGEventType type) {
    return CGEventSourceCounterForEventType(kCGEventSourceStateHIDSystemState, type);
}

static uint32_t pc_keyboard_counter(void) {
    return pc_counter(kCGEventKeyDown) + pc_counter(kCGEventFlagsChanged);
}

static uint32_t pc_mouse_action_counter(void) {
    return
        pc_counter(kCGEventLeftMouseDown) + pc_counter(kCGEventLeftMouseUp) +
        pc_counter(kCGEventRightMouseDown) + pc_counter(kCGEventRightMouseUp) +
        pc_counter(kCGEventOtherMouseDown) + pc_counter(kCGEventOtherMouseUp) +
        pc_counter(kCGEventLeftMouseDragged) + pc_counter(kCGEventRightMouseDragged) +
        pc_counter(kCGEventOtherMouseDragged) + pc_counter(kCGEventScrollWheel);
}

static uint32_t pc_mouse_counter(void) {
    return pc_counter(kCGEventMouseMoved) + pc_mouse_action_counter();
}

static uint32_t pc_presence_counter(void) {
    // Plain pointer motion is intentionally excluded. Some mice produce an
    // occasional movement event while nobody is present, which used to defer
    // the camera check indefinitely.
    return pc_keyboard_counter() + pc_mouse_action_counter();
}

static bool pc_preflight(void) { return CGPreflightListenEventAccess(); }
static bool pc_request(void) { return CGRequestListenEventAccess(); }
*/
import "C"
import "sync"

var requestOnce sync.Once

func readCounters() counters {
	return counters{
		keyboard: uint32(C.pc_keyboard_counter()),
		mouse:    uint32(C.pc_mouse_counter()),
		presence: uint32(C.pc_presence_counter()),
	}
}

func PreflightAccess() bool { return bool(C.pc_preflight()) }
func RequestAccess() bool   { return bool(C.pc_request()) }

func RequestAccessOnce() { requestOnce.Do(func() { RequestAccess() }) }
