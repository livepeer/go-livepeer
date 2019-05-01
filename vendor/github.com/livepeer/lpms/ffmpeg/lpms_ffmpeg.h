#ifndef _LPMS_FFMPEG_H_
#define _LPMS_FFMPEG_H_

#include <libavutil/hwcontext.h>
#include <libavutil/rational.h>

// LPMS specific errors
extern const int lpms_ERR_INPUT_PIXFMT;
extern const int lpms_ERR_FILTERS;

typedef struct {
  char *fname;
  char *vencoder;
  char *vfilters;
  int w, h, bitrate;
  AVRational fps;
} output_params;

typedef struct {
  char *fname;

  // Optional hardware acceleration
  enum AVHWDeviceType hw_type;
  char *device;
} input_params;

void lpms_init();
int  lpms_rtmp2hls(char *listen, char *outf, char *ts_tmpl, char *seg_time, char *seg_start);
int  lpms_transcode(input_params *inp, output_params *params, int nb_outputs);

#endif // _LPMS_FFMPEG_H_
