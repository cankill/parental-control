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

int pc_analyze_face_touch(const char *path, int *faces, int *hands, double *score, char **error_message) {
	@autoreleasepool {
		if (path == NULL || faces == NULL || hands == NULL || score == NULL) {
			return pc_vision_error(@"invalid face-touch analysis arguments", error_message);
		}
		*faces = 0;
		*hands = 0;
		*score = 0;
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
			*faces = (int)faceResults.count;
			*hands = (int)handResults.count;

			NSArray *jointNames = @[
				VNHumanHandPoseObservationJointNameThumbTip,
				VNHumanHandPoseObservationJointNameIndexTip,
				VNHumanHandPoseObservationJointNameMiddleTip,
				VNHumanHandPoseObservationJointNameRingTip,
				VNHumanHandPoseObservationJointNameLittleTip,
				VNHumanHandPoseObservationJointNameIndexMCP,
				VNHumanHandPoseObservationJointNameMiddleMCP,
				VNHumanHandPoseObservationJointNameRingMCP,
				VNHumanHandPoseObservationJointNameLittleMCP
			];

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

				double radiusX = face.boundingBox.size.width * 0.33;
				double radiusY = face.boundingBox.size.height * 0.27;
				for (VNHumanHandPoseObservation *hand in handResults) {
					for (NSString *jointName in jointNames) {
						NSError *pointError = nil;
						VNRecognizedPoint *point = [hand recognizedPointForJointName:jointName error:&pointError];
						if (point == nil || pointError != nil || point.confidence < 0.3) {
							continue;
						}
						double dx = (point.location.x - chinX) / radiusX;
						double dy = (point.location.y - chinY) / radiusY;
						double normalizedDistance = hypot(dx, dy);
						if (normalizedDistance > 1.0) {
							continue;
						}
						double proximity = 1.0 - normalizedDistance;
						double confidence = fmin(face.confidence, point.confidence);
						double candidate = 0.65 * proximity + 0.35 * confidence;
						if (candidate > *score) {
							*score = candidate;
						}
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
