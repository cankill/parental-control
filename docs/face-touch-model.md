# Face-touch Core ML model

The face-touch detector has two local stages:

1. Apple Vision locates a pinch pose near the chin.
2. `ChinPinchClassifier.mlmodelc` classifies the chin-area crop as `watch` or
   `ignore`.

Photos, labels, prepared crops, and the trained model remain on the local Macs.
Do not commit any of these generated artifacts.

## Train a new model

Copy the private `database/face-touch` directory from the monitored Mac to a
temporary local directory. Then build and run the trainer on a Mac with full
Xcode installed:

```bash
xcrun swiftc \
  -framework CreateML \
  -framework Vision \
  -framework CoreGraphics \
  -framework ImageIO \
  -framework UniformTypeIdentifiers \
  tools/facetouch-trainer/main.swift \
  -o /tmp/ifocus-facetouch-trainer

/tmp/ifocus-facetouch-trainer \
  /path/to/database/face-touch \
  /tmp/ifocus-face-touch-model
```

The trainer creates deterministic training and test sets, crops the lower-face
region, prints training/validation/test metrics, and writes both
`ChinPinchClassifier.mlmodel` and `ChinPinchClassifier.mlmodelc`.

## Install

Copy the compiled directory to:

```text
database/face-touch/models/ChinPinchClassifier.mlmodelc
```

Restart the agent. `/chin` should report `ChinPinchClassifier v1 — active`.
The geometric score remains a prefilter; Telegram messages are created only
when the model's `watch` probability reaches `FACE_TOUCH_MODEL_THRESHOLD_PERCENT`.

Keep labeling the messages. Retrain in batches and replace the compiled model
only after its untouched test metrics improve. Favor `watch` precision when
choosing the notification threshold so false notifications stay rare.
