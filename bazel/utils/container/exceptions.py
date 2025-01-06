"""Common custom exceptions used by bazel build rules"""


class UnofficialBuildException(Exception):
    def __init__(self, image_name):
        super().__init__()
        self.image_name = image_name

    def __str__(self):
        return f"{self.image_name} was not built on a clean master branch"
