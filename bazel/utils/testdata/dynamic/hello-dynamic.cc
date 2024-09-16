#include <iostream>
#include <gtest/gtest.h>

int main(int argc, char** argv) {
  int status = 0;

  // Ensure a function from gtest is included.
  // Using argc so the compiler cannot optimize it out.
  EXPECT_EQ(1, argc);

  // Return a non-0 status is argc > 1, to test arg propagation.
  if (argc > 1) status = 1;

  // Return a non-0 if a variable TEST_ENV_PROPAGATION is seen with
  // a specific value. Used to verify env propagation.
  const char* env = getenv("TEST_ENV_PROPAGATION");
  if (env && strcmp(env, "42")) status = 3;

  std::cout << "Hello, world!" << std::endl;
  return status;
}
