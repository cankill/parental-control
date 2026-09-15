#import <Foundation/Foundation.h>
#import <ImageIO/ImageIO.h>
#import <Vision/Vision.h>
#include <math.h>

#include <stdlib.h>
#include <string.h>

#include "detector.h"

static int pc_vision_error(NSString *message, char **error_message) {
    if (error_message != NULL) {
        const char *utf8 = message.UTF8String;
        *error_message = strdup(utf8 != NULL ? utf8 : "unknown Vision error");
    }
    return -1;
}

int pc_analyze_presence(const char *path, int *humans, int *faces, char **error_message) {
    @autoreleasepool {
        if (path == NULL || humans == NULL || faces == NULL) {
            return pc_vision_error(@"invalid presence analysis arguments", error_message);
        }
        *humans = 0;
        *faces = 0;
        if (error_message != NULL) {
            *error_message = NULL;
        }

        NSString *imagePath = [NSString stringWithUTF8String:path];
        NSURL *imageURL = [NSURL fileURLWithPath:imagePath];
        CGImageSourceRef source = CGImageSourceCreateWithURL((__bridge CFURLRef)imageURL, NULL);
        if (source == NULL) {
            return pc_vision_error(@"could not read image", error_message);
        }
        CGImageRef image = CGImageSourceCreateImageAtIndex(source, 0, NULL);
        CFRelease(source);
        if (image == NULL) {
            return pc_vision_error(@"could not decode image", error_message);
        }

        VNDetectHumanRectanglesRequest *humanRequest = [[VNDetectHumanRectanglesRequest alloc] init];
        humanRequest.upperBodyOnly = YES;
        VNDetectFaceRectanglesRequest *faceRequest = [[VNDetectFaceRectanglesRequest alloc] init];
        VNImageRequestHandler *handler = [[VNImageRequestHandler alloc] initWithCGImage:image options:@{}];

        NSError *requestError = nil;
        BOOL success = [handler performRequests:@[humanRequest, faceRequest] error:&requestError];
        if (success) {
            *humans = (int)humanRequest.results.count;
            *faces = (int)faceRequest.results.count;
        }

        [handler release];
        [faceRequest release];
        [humanRequest release];
        CGImageRelease(image);

        if (!success) {
            return pc_vision_error(requestError.localizedDescription, error_message);
        }
        return 0;
    }
}

static CGImageRef pc_load_image(const char *path, char **error_message) {
	NSString *imagePath = [NSString stringWithUTF8String:path];
	NSURL *imageURL = [NSURL fileURLWithPath:imagePath];
	CGImageSourceRef source = CGImageSourceCreateWithURL((__bridge CFURLRef)imageURL, NULL);
	if (source == NULL) {
		pc_vision_error(@"could not read image", error_message);
		return NULL;
	}
	CGImageRef image = CGImageSourceCreateImageAtIndex(source, 0, NULL);
	CFRelease(source);
	if (image == NULL) {
		pc_vision_error(@"could not decode image", error_message);
	}
	return image;
}

static VNRecognizedPoint *pc_hand_point(VNHumanHandPoseObservation *hand, NSString *jointName) {
	NSError *pointError = nil;
	VNRecognizedPoint *point = [hand recognizedPointForJointName:jointName error:&pointError];
	return pointError == nil ? point : nil;
}

static double pc_point_distance(VNRecognizedPoint *first, VNRecognizedPoint *second) {
	return hypot(first.location.x - second.location.x, first.location.y - second.location.y);
}

static void pc_consider_pinch(
	pc_face_touch_result *result,
	VNFaceObservation *face,
	double chinX,
	double chinY,
	VNRecognizedPoint *first,
	VNRecognizedPoint *second,
	double handScale,
	double maximumNormalizedTipDistance,
	double landmarkConfidence,
	int poseMode,
	double thumbConfidence,
	double indexConfidence,
	double middleConfidence
) {
	double tipDistance = pc_point_distance(first, second);
	double maximumTipDistance = handScale * maximumNormalizedTipDistance;
	if (maximumTipDistance <= 0 || tipDistance > maximumTipDistance) {
		return;
	}

	double pinchX = (first.location.x + second.location.x) / 2.0;
	double pinchY = (first.location.y + second.location.y) / 2.0;
	double verticalOffset = (pinchY - chinY) / face.boundingBox.size.height;
	if (verticalOffset < -0.26 || verticalOffset > 0.24) {
		return;
	}
	double radiusX = face.boundingBox.size.width * 0.42;
	double radiusY = face.boundingBox.size.height * 0.28;
	double dx = (pinchX - chinX) / radiusX;
	double dy = (pinchY - chinY) / radiusY;
	double normalizedChinDistance = hypot(dx, dy);
	if (normalizedChinDistance > 1.0) {
		return;
	}

	double chinProximity = 1.0 - normalizedChinDistance;
	double pinchCloseness = 1.0 - tipDistance / maximumTipDistance;
	double confidence = fmin(face.confidence, landmarkConfidence);
	// Geometry gates define the pose. The base score keeps valid but partially
	// occluded fingertip clusters observable during dataset collection.
	double candidate = 0.35 + 0.35 * chinProximity + 0.20 * pinchCloseness + 0.10 * confidence;
	if (candidate <= result->score) {
		return;
	}
	result->pose_mode = poseMode;
	result->score = candidate;
	result->chin_proximity = chinProximity;
	result->pinch_closeness = pinchCloseness;
	result->landmark_confidence = confidence;
	result->normalized_tip_distance = tipDistance / handScale;
	result->hand_scale = handScale;
	result->thumb_confidence = thumbConfidence;
	result->index_confidence = indexConfidence;
	result->middle_confidence = middleConfidence;
}

int pc_analyze_face_touch(const char *path, pc_face_touch_result *result, char **error_message) {
	@autoreleasepool {
		if (path == NULL || result == NULL) {
			return pc_vision_error(@"invalid face-touch analysis arguments", error_message);
		}
		memset(result, 0, sizeof(*result));
		if (error_message != NULL) {
			*error_message = NULL;
		}

		CGImageRef image = pc_load_image(path, error_message);
		if (image == NULL) {
			return -1;
		}

		VNDetectFaceLandmarksRequest *faceRequest = [[VNDetectFaceLandmarksRequest alloc] init];
		VNDetectHumanHandPoseRequest *handRequest = [[VNDetectHumanHandPoseRequest alloc] init];
		handRequest.maximumHandCount = 2;
		VNImageRequestHandler *handler = [[VNImageRequestHandler alloc] initWithCGImage:image options:@{}];

		NSError *requestError = nil;
		BOOL success = [handler performRequests:@[faceRequest, handRequest] error:&requestError];
		if (success) {
			NSArray<VNFaceObservation *> *faceResults = faceRequest.results;
			NSArray<VNHumanHandPoseObservation *> *handResults = handRequest.results;
			result->faces = (int)faceResults.count;
			result->hands = (int)handResults.count;

			for (VNFaceObservation *face in faceResults) {
				VNFaceLandmarkRegion2D *contour = face.landmarks.faceContour;
				if (contour == nil || contour.pointCount == 0 || face.boundingBox.size.width <= 0 || face.boundingBox.size.height <= 0) {
					continue;
				}
				const CGPoint *contourPoints = contour.normalizedPoints;
				double chinX = face.boundingBox.origin.x + contourPoints[0].x * face.boundingBox.size.width;
				double chinY = face.boundingBox.origin.y + contourPoints[0].y * face.boundingBox.size.height;
				for (NSUInteger index = 1; index < contour.pointCount; index++) {
					double imageY = face.boundingBox.origin.y + contourPoints[index].y * face.boundingBox.size.height;
					if (imageY < chinY) {
						chinX = face.boundingBox.origin.x + contourPoints[index].x * face.boundingBox.size.width;
						chinY = imageY;
					}
				}

				for (VNHumanHandPoseObservation *hand in handResults) {
					VNRecognizedPoint *thumb = pc_hand_point(hand, VNHumanHandPoseObservationJointNameThumbTip);
					VNRecognizedPoint *index = pc_hand_point(hand, VNHumanHandPoseObservationJointNameIndexTip);
					VNRecognizedPoint *middle = pc_hand_point(hand, VNHumanHandPoseObservationJointNameMiddleTip);
					VNRecognizedPoint *wrist = pc_hand_point(hand, VNHumanHandPoseObservationJointNameWrist);
					VNRecognizedPoint *middleMCP = pc_hand_point(hand, VNHumanHandPoseObservationJointNameMiddleMCP);
					double thumbConfidence = thumb != nil ? thumb.confidence : 0;
					double indexConfidence = index != nil ? index.confidence : 0;
					double middleConfidence = middle != nil ? middle.confidence : 0;

					double handScale = face.boundingBox.size.width;
					if (wrist != nil && middleMCP != nil && wrist.confidence >= 0.3 && middleMCP.confidence >= 0.3) {
						handScale = fmax(handScale, pc_point_distance(wrist, middleMCP));
					}

					if (thumb != nil && index != nil && thumbConfidence >= 0.3 && indexConfidence >= 0.3) {
						pc_consider_pinch(result, face, chinX, chinY, thumb, index, handScale, 0.16,
							fmin(thumbConfidence, indexConfidence), 1,
							thumbConfidence, indexConfidence, middleConfidence);
					}
					// When the thumb is partially hidden against the chin, Vision often
					// maps the two visible fingertips to index and middle. Keep this
					// fallback narrow and disable it for a confidently visible thumb.
					if (thumbConfidence < 0.45 && index != nil && middle != nil && indexConfidence >= 0.25 && middleConfidence >= 0.25) {
						pc_consider_pinch(result, face, chinX, chinY, index, middle, handScale, 0.20,
							fmin(indexConfidence, middleConfidence), 2,
							thumbConfidence, indexConfidence, middleConfidence);
					}
				}
			}
		}

		[handler release];
		[handRequest release];
		[faceRequest release];
		CGImageRelease(image);

		if (!success) {
			return pc_vision_error(requestError.localizedDescription, error_message);
		}
		return 0;
	}
}
