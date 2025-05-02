#include "bestie/utils/googlebench/bestie_reporter.hh"

#include <google/protobuf/text_format.h>

#include <chrono>
#include <fstream>

#include "google/protobuf/io/zero_copy_stream.h"
#include "google/protobuf/io/zero_copy_stream_impl.h"

namespace bestie {

std::string MetricsPath(const std::string_view filename, std::string_view path) {
  return absl::StrCat(path, "/", filename, ".metrics.pb");
}

std::string MetricsPath(const std::string_view filename) {
  // getenv is not thread safe, static variable initialization is.
  // Also, try to discourage changing the environment variables at run time.
  static const std::string path([]() {
    if (const char* path = std::getenv("TEST_UNDECLARED_OUTPUTS_DIR"); path) return path;
    if (const char* path = std::getenv("TMPDIR"); path) return path;

    return "/tmp";
  }());

  return MetricsPath(filename, path);
}

std::string MetricsPath(int attempt, const std::string_view filename) {
  if (attempt <= 0) return MetricsPath(filename);
  return MetricsPath(absl::StrFormat("%s%03d", filename, attempt));
}

static void AddTag(bestie::proto::TestMetric* metric, const std::string& key,
                   const std::string& value) {
  auto* tag = metric->add_tags();
  tag->set_key(key);
  tag->set_value(value);
}

static bestie::proto::TestMetric* AddMetric(bestie::proto::TestMetrics* metrics,
                                            const std::string& name, double value,
                                            const auto&... prototype) {
  auto* metric = metrics->add_metrics();

  metric->set_metricname(name);
  metric->set_value(value);

  (metric->MergeFrom(prototype), ...);

  return metric;
}

bool Reporter::ReportContext(const Context& context) {
  int64_t epoch = std::chrono::duration_cast<std::chrono::nanoseconds>(
                      std::chrono::system_clock::now().time_since_epoch())
                      .count();

  context_.Clear();
  context_.set_timestamp(epoch);

  AddTag(&context_, "context__sys_info__name", context.sys_info.name);
  AddTag(&context_, "context__executable_name", context.executable_name);

  AddMetric(&metrics_, "context__cpu_info__num_cpus", context.cpu_info.num_cpus, context_);
  AddMetric(&metrics_, "context__cpu_info__cycles_per_second", context.cpu_info.cycles_per_second,
            context_);
  for (std::size_t i = 0; i < std::size(context.cpu_info.load_avg); ++i) {
    AddMetric(&metrics_, absl::StrFormat("context__cpu_info__load_avg__%d", i),
              context.cpu_info.load_avg[i], context_);
  }

  return true;
}

void Reporter::ReportRuns(const std::vector<Run>& reports) {
  for (const auto& run : reports) {
    bestie::proto::TestMetric run_context;

    AddTag(&run_context, "run__benchmark_name", run.benchmark_name());
    if (!run.report_label.empty()) AddTag(&run_context, "run__report_label", run.report_label);
    if (run.skipped) {
      const char* reason =
          run.skipped == benchmark::internal::SkippedWithError ? "error" : "message";
      AddTag(&run_context, "run__skipped", reason);
      AddTag(&run_context, "run__skip_message", run.skip_message);
    }

    AddMetric(&metrics_, "run__iterations", run.iterations, run_context, context_);
    AddMetric(&metrics_, "run__cpu_accumulated_time", run.cpu_accumulated_time, run_context,
              context_);
    AddMetric(&metrics_, "run__real_accumulated_time", run.real_accumulated_time, run_context,
              context_);

    AddMetric(&metrics_, "run__adjusted_cpu_time", run.GetAdjustedCPUTime(), run_context, context_);
    AddMetric(&metrics_, "run__adjusted_real_time", run.GetAdjustedRealTime(), run_context,
              context_);
    AddMetric(&metrics_, "run__max_heapbytes_used", run.max_heapbytes_used, run_context, context_);
    AddMetric(&metrics_, "run__allocs_per_iter", run.allocs_per_iter, run_context, context_);

    AddMetric(&metrics_, "run__memory_result__num_allocs", run.memory_result.num_allocs,
              run_context, context_);
    AddMetric(&metrics_, "run__memory_result__max_bytes_used", run.memory_result.max_bytes_used,
              run_context, context_);
    AddMetric(&metrics_, "run__memory_result__total_allocated_bytes",
              run.memory_result.total_allocated_bytes, run_context, context_);
    AddMetric(&metrics_, "run__memory_result__net_heap_growth",
              run.memory_result.net_heap_growth, run_context, context_);

    for (const auto& [name, counter] : run.counters) {
      auto* metric = AddMetric(&metrics_, absl::StrFormat("run__counters__%s", name), counter.value,
                               run_context, context_);
      AddTag(metric, "unit", counter.oneK == benchmark::Counter::kIs1000 ? "1000" : "1024");
      AddTag(metric, "flags", absl::StrFormat("%d", counter.flags));
    }
  }
}

// TODO(carlo): 04/29/2025 - get rid of this once we update our development container or
// start using an hermetic toolchain in the enkit/ repository that's newer than gcc-10.
#ifdef __cpp_lib_ios_noreplace
#define bestie_noreplace std::ios::noreplace
#else
#define bestie_noreplace std::ios::openmode(0)
#endif

bool OutputBazel(const google::protobuf::Message& message, std::ostream& ostream,
                 std::ostream& estream, int attempts) {
  std::string filename;
  for (int attempt = 0; attempt < attempts; ++attempt) {
    filename = MetricsPath();
    std::ofstream filestream(filename,
                             std::ios::out | std::ios::binary | std::ios::trunc | bestie_noreplace);
    if (filestream.is_open()) return message.SerializeToOstream(&filestream);

    if (errno != EEXIST) {
      estream << "ERROR: saving benchmark results in " << filename
              << " failed with errno: " << errno << "\n";
      return false;
    }
  }

  estream << "ERROR: failed to find unique file name in " << attempts
          << " attempts - last name attempted " << filename << "\n";
  return false;
}

bool OutputBazel(const google::protobuf::Message& message, std::ostream& ostream,
                 std::ostream& estream) {
  return OutputBazel(message, ostream, estream, kDefaultAttempts);
}

bool OutputBinary(const google::protobuf::Message& message, std::ostream& ostream, std::ostream&) {
  return message.SerializeToOstream(&ostream);
}

bool OutputHuman(const google::protobuf::Message& message, std::ostream& ostream, std::ostream&) {
  google::protobuf::io::OstreamOutputStream output(&ostream);
  return google::protobuf::TextFormat::Print(message, &output);
}

bool OutputDefault(const google::protobuf::Message& message, std::ostream& ostream,
                   std::ostream& estream) {
  // & instead of && on purpose - avoid short circuit logic: if human output fails, still try the
  // bazel output, and the other way around. But only succeed if both succeed.
  return OutputHuman(message, ostream, estream) & OutputBazel(message, ostream, estream);
}

void Reporter::Finalize() {
  auto& errstream = GetErrorStream();
  if (!outputter_(metrics_, GetOutputStream(), errstream)) {
    errstream << "ERROR: benchmark test results were NOT written\n";
  }
}

}  // namespace bestie
