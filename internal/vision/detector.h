#ifndef PARENTCONTROL_VISION_DETECTOR_H
#define PARENTCONTROL_VISION_DETECTOR_H

int pc_analyze_presence(const char *path, int *humans, int *faces, char **error_message);

typedef struct {
    int faces;
    int hands;
    int pose_mode;
    double score;
    double chin_proximity;
    double pinch_closeness;
    double landmark_confidence;
    double normalized_tip_distance;
    double hand_scale;
    double thumb_confidence;
    double index_confidence;
    double middle_confidence;
} pc_face_touch_result;

int pc_analyze_face_touch(const char *path, pc_face_touch_result *result, char **error_message);

#endif
