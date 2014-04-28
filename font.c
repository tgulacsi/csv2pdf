#include "runtime.h"
extern byte _brsrc_font[], _ersrc_font;

/* func get_font() []byte */
void ·get_font(Slice a) {
  a.array = _brsrc_font;
  a.len = a.cap = &_ersrc_font - _brsrc_font;
  FLUSH(&a);
}
