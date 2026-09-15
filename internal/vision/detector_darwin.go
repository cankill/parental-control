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
// proximity of a thumb-and-index pinch pose to the chin contour.
type FaceTouchDetection struct {
	Faces                 int
	Hands                 int
	PoseMode              int
	Score                 float64
	ChinProximity         float64
	PinchCloseness        float64
	LandmarkConfidence    float64
	NormalizedTipDistance float64
	HandScale             float64
	ThumbConfidence       float64
	IndexConfidence       float64
	MiddleConfidence      float64
}

// AnalyzeFaceTouch detects face landmarks and hand pose locally, then scores
// how closely a thumb-and-index pinch pose overlaps the chin region.
func AnalyzeFaceTouch(path string) (FaceTouchDetection, error) {
	cPath := C.CString(path)
	defer C.free(unsafe.Pointer(cPath))

	var result C.pc_face_touch_result
	var errorMessage *C.char
	if C.pc_analyze_face_touch(cPath, &result, &errorMessage) != 0 {
		if errorMessage == nil {
			return FaceTouchDetection{}, fmt.Errorf("Apple Vision face-touch analysis failed")
		}
		defer C.free(unsafe.Pointer(errorMessage))
		return FaceTouchDetection{}, fmt.Errorf("Apple Vision face-touch analysis failed: %s", C.GoString(errorMessage))
	}
	return FaceTouchDetection{
		Faces: int(result.faces), Hands: int(result.hands), PoseMode: int(result.pose_mode),
		Score: float64(result.score), ChinProximity: float64(result.chin_proximity),
		PinchCloseness: float64(result.pinch_closeness), LandmarkConfidence: float64(result.landmark_confidence),
		NormalizedTipDistance: float64(result.normalized_tip_distance), HandScale: float64(result.hand_scale),
		ThumbConfidence: float64(result.thumb_confidence), IndexConfidence: float64(result.index_confidence),
		MiddleConfidence: float64(result.middle_confidence),
	}, nil
}
