//go:build darwin && cgo

package vision

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework Foundation -framework CoreGraphics -framework ImageIO -framework Vision
#include <stdlib.h>
#include "detector.h"
*/
import "C"

import (
	"fmt"
	"unsafe"
)

// Detection is a privacy-preserving presence result. It contains counts only;
// the detector performs no recognition and retains no biometric information.
type Detection struct {
	Humans int
	Faces  int
}

func (d Detection) Present() bool {
	return d.Humans > 0 || d.Faces > 0
}

// AnalyzeImage detects human upper bodies and faces using Apple's local Vision
// framework. Image lifecycle remains the caller's responsibility.
func AnalyzeImage(path string) (Detection, error) {
	cPath := C.CString(path)
	defer C.free(unsafe.Pointer(cPath))

	var humans C.int
	var faces C.int
	var errorMessage *C.char
	if C.pc_analyze_presence(cPath, &humans, &faces, &errorMessage) != 0 {
		if errorMessage == nil {
			return Detection{}, fmt.Errorf("Apple Vision presence analysis failed")
		}
		defer C.free(unsafe.Pointer(errorMessage))
		return Detection{}, fmt.Errorf("Apple Vision presence analysis failed: %s", C.GoString(errorMessage))
	}
	return Detection{Humans: int(humans), Faces: int(faces)}, nil
}

// FaceTouchDetection is a frame-level geometric score. Score is not a
// calibrated probability: it combines Vision landmark confidence with the
// normalized distance between recognized hand joints and the chin contour.
type FaceTouchDetection struct {
	Faces int
	Hands int
	Score float64
}

// AnalyzeFaceTouch detects face landmarks and hand pose locally, then scores
// how closely a recognized hand joint overlaps the chin region.
func AnalyzeFaceTouch(path string) (FaceTouchDetection, error) {
	cPath := C.CString(path)
	defer C.free(unsafe.Pointer(cPath))

	var faces C.int
	var hands C.int
	var score C.double
	var errorMessage *C.char
	if C.pc_analyze_face_touch(cPath, &faces, &hands, &score, &errorMessage) != 0 {
		if errorMessage == nil {
			return FaceTouchDetection{}, fmt.Errorf("Apple Vision face-touch analysis failed")
		}
		defer C.free(unsafe.Pointer(errorMessage))
		return FaceTouchDetection{}, fmt.Errorf("Apple Vision face-touch analysis failed: %s", C.GoString(errorMessage))
	}
	return FaceTouchDetection{Faces: int(faces), Hands: int(hands), Score: float64(score)}, nil
}
