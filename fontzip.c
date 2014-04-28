#include "runtime.h"
extern byte _brsrc_fontzip[], _ersrc_fontzip;

/* func get_fontzip() []byte */
void ·get_fontzip(Slice a) {
  a.array = _brsrc_fontzip;
  a.len = a.cap = &_ersrc_fontzip - _brsrc_fontzip;
  FLUSH(&a);
}
