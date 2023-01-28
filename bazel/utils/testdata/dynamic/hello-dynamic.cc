#include <iostream>
#include <gtest/gtest.h>

int main(int argc, char** argv) {
  // Ensure a function from gtest is included.
  // Using argc so the compiler cannot optimize it out.
  EXPECT_EQ(1, argc);

  std::cout << "Hello, world!" << std::endl;
  return 0;
}
