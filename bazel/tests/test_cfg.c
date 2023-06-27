#include <stdio.h>
#include <assert.h>

int main(void) {
  int unused;
  assert((unused = 3) == 3);
  fprintf(stderr, "hello world");
  return 0;
}
