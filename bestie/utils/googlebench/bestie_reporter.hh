#ifndef _BESTIE_UTILS_GOOGLEBENCH_BESTIE_REPORTER_HH_
#define _BESTIE_UTILS_GOOGLEBENCH_BESTIE_REPORTER_HH_

// This file provides a "Reporter" usable from the google benchmark library.
// Reporters are used to save the results of a benchmark.
//
// By default, google bench is capable of saving the results in a human
// readable format, typically printed on the console, in json format, or
// in CSV format (marked for deprecation as of 2025).
//
// The class in this file saves the output in protocol buffer format
// usable by bestie, bestie/proto/test_metrics.pb.
//
// This file also provide a few utility functions to - for example - find
// the correct path where to save those files, or to output the metrics
// in text proto format for console printing/debugging..

#include <functional>
#include <ostream>

#include "benchmark/benchmark.h"
#include "bestie/proto/test_metrics.pb.h"

namespace bestie {

// bestie will process any file with extension ".metrics.proto".
// The name of the file does not really matter, by convention we use "test",
// for "test.metrics.proto".
inline constexpr const char* kDefaultFilename = "test";

// If multiple benchmarks are run in the same test invocation (uncommon),
// there's the risk of overwriting "test.metrics.proto".
// By default, the code in this file will not overwrite. Instead, it will
// attempt to find a unique file name by appending an integer to the filename,
// up to kDefaultAttempts. Example: "test023.metrics.proto".
inline constexpr int kDefaultAttempts = 50;

// Return a path where to store metrics for bestie to process them.
//
// This variant can be invoked without parameters, returns a path like:
//   $TEST_UNDECLARED_OUTPUT_DIR/test.metrics.pb
// when TEST_UNDECLARED_OUTPUT_DIR exists, or $TMPDIR or /tmp like:
//   /tmp/test.metrics.pb
// if it does not exist.
//
// Optionally, it can be provided a file name to override "test".
extern std::string MetricsPath(const std::string_view filename = kDefaultFilename);

// Same as above, but takes an optional "attempt" parameter.
//
// If attempt is 0, then the filename returned is "$.../test.metrics.pb" just like
// described above.
// If attempt is != 0, then the filename returned is "$.../test003.metrics.pb", for
// example (003 representing the value of attempts).
extern std::string MetricsPath(int attempt, const std::string_view filename = kDefaultFilename);

// Same as above, but both path (the tmp directory) and filename must be specified.
// You should generally prefer one of the other variants.
extern std::string MetricsPath(const std::string_view filename, std::string_view path);

// The google bench reporter can be configured to save the output in a variety
// of ways (typically, via command line flags). Those flags will typically initialize
// an output stream (the first ostream parameter) and an error stream (the second one).
//
// The metrics need to be output on the first ostream.
//
// This ostream is by default printed on the console, so it is included in the benchmark
// output on the screen for humans to read.
// Bestie, however, requires the output to be saved in a test.metrics.pb file, in binary
// format.
//
// But the googlebench library supports two different reporters, a "main one", and one
// to save it in a file.
//
// Depending on how you use the google bench library, you may want to tune how
// the output is saved and reported.
//
// You can use one of the Output functions here (or define your own) to customize
// this output.

// OutputDefault tries to mimic the behavior of googlebench the best way it can.
//
// It will print the metrics in text format on the stream provided, while saving
// them in binary format in the TEST_UNDECLARED_OUTPUT_DIR so that bestie can
// find those metrics and archive them.
//
// It's implemented by invoking OutputHuman and OutputBazel defined below.
bool OutputDefault(const google::protobuf::Message& message, std::ostream&, std::ostream&);

// Only ouptuts the metrics in text format in the specified stream.
bool OutputHuman(const google::protobuf::Message& message, std::ostream& ostream,
                 std::ostream& estream);

// Only ouptuts the metrics in binary format in the specified stream.
bool OutputBinary(const google::protobuf::Message& message, std::ostream& ostream,
                  std::ostream& estream);

// Ouptuts the metrics in binary format in the TEST_UNDECLARED_OUTPUT_DIR (the specified stream is
// ignored). The (optional) attempts parameter allows to customize the "attempts" behavior described
// in the MetricsPath functions.
bool OutputBazel(const google::protobuf::Message& message, std::ostream&, std::ostream&,
                 int attempts);
bool OutputBazel(const google::protobuf::Message& message, std::ostream&, std::ostream& estream);

// This is a "reporter" usable in the googlebench library.
class Reporter : public ::benchmark::BenchmarkReporter {
 public:
  using Outputter = std::function<bool(const google::protobuf::Message& message,
                                       std::ostream& stream, std::ostream& estream)>;

  Reporter() = default;
  explicit Reporter(Outputter outputter) : outputter_(outputter) {}

  bool ReportContext(const Context& context) override;
  void ReportRuns(const std::vector<Run>& reports) override;
  void Finalize() override;

 private:
  const Outputter outputter_ = OutputDefault;

  bestie::proto::TestMetric context_;
  bestie::proto::TestMetrics metrics_;
};

}  // namespace bestie

#endif
