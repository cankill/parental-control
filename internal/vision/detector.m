#import <Foundation/Foundation.h>
#import <ImageIO/ImageIO.h>
#import <Vision/Vision.h>

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
