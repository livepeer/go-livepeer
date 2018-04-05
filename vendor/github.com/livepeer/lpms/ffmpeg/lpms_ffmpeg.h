#include <libavutil/rational.h>

typedef struct {
  char *fname;
  int w, h, bitrate;
  AVRational fps;
} output_params;

void lpms_init();
void lpms_deinit();
int  lpms_rtmp2hls(char *listen, char *outf, char *ts_tmpl, char *seg_time);
int  lpms_transcode(char *inp, output_params *params, int nb_outputs);
int  lpms_length(char *inp, int ts_max, int packet_max);
