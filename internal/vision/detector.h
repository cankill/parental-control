#ifndef PARENTCONTROL_VISION_DETECTOR_H
#define PARENTCONTROL_VISION_DETECTOR_H

int pc_analyze_presence(const char *path, int *humans, int *faces, char **error_message);
int pc_analyze_face_touch(const char *path, int *faces, int *hands, double *score, char **error_message);

#endif
