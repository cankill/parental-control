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
