"""Merges user-defined container labels with bazel stamp build info"""
# third party libraries
from absl import app, flags

# enfabrica libraries
from bazel.utils.container import stamp

FLAGS = flags.FLAGS
flags.DEFINE_string("user_labels", None, "User-defined container labels file with key-value pairs with '=' as delimiters")
flags.DEFINE_string("output", None, "Output file with bazel --stamp info appeneded at the end")


def container_stamper(user_labels, output):
    stamp_info = stamp.get_buildstamp_values()
    with open(user_labels, "r", encoding="utf-8") as fdin, open(output, "w", encoding="utf-8") as fdout:
        for line in fdin.readlines():
            fdout.write(line)
        for key in ["STABLE_GIT_MASTER_SHA", "GIT_BRANCH", "BUILD_USER", "BUILD_TIME"]:
            if stamp_info.get(key):
                fdout.write(f"{key}={stamp_info.get(key)}\n")
        fdout.write(f"CLEAN_BUILD={stamp.is_clean(stamp_info)}\n")
        fdout.write(f"OFFICIAL_BUILD={stamp.is_official(stamp_info)}\n")


def main(argv):
    del argv
    container_stamper(FLAGS.user_labels, FLAGS.output)


if __name__ == "__main__":
    flags.mark_flag_as_required("user_labels")
    flags.mark_flag_as_required("output")
    app.run(main)
