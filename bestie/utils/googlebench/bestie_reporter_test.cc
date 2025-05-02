#include "bestie/utils/googlebench/bestie_reporter.hh"

#include <gmock/gmock.h>
#include <google/protobuf/text_format.h>
#include <google/protobuf/util/message_differencer.h>
#include <gtest/gtest.h>

#include <fstream>
#include <iostream>

TEST(Bestie, MetricsPath) {
  EXPECT_THAT(bestie::MetricsPath(),
              testing::ContainsRegex("bestie/utils/googlebench.*/test.metrics.pb"));
  EXPECT_THAT(bestie::MetricsPath("freedom"),
              testing::ContainsRegex("bestie/utils/googlebench.*/freedom.metrics.pb"));
  EXPECT_EQ(bestie::MetricsPath("freedom", "/tmp/mypath"), "/tmp/mypath/freedom.metrics.pb");
  EXPECT_THAT(bestie::MetricsPath(0),
              testing::ContainsRegex("bestie/utils/googlebench.*/test.metrics.pb"));
  EXPECT_THAT(bestie::MetricsPath(1),
              testing::ContainsRegex("bestie/utils/googlebench.*/test001.metrics.pb"));
  EXPECT_THAT(bestie::MetricsPath(123, "freedom"),
              testing::ContainsRegex("bestie/utils/googlebench.*/freedom123.metrics.pb"));
}

const std::string kExpectedMessageText = R"(
metrics {
  metricname: "context__cpu_info__num_cpus"
  tags {
    key: "context__sys_info__name"
    value: "<hostname>"
  }
  tags {
    key: "context__executable_name"
    value: "<path-of-binary>"
  }
}
metrics {
  metricname: "context__cpu_info__cycles_per_second"
  tags {
    key: "context__sys_info__name"
    value: "<hostname>"
  }
  tags {
    key: "context__executable_name"
    value: "<path-of-binary>"
  }
}
metrics {
  metricname: "context__cpu_info__load_avg__0"
  tags {
    key: "context__sys_info__name"
    value: "<hostname>"
  }
  tags {
    key: "context__executable_name"
    value: "<path-of-binary>"
  }
}
metrics {
  metricname: "context__cpu_info__load_avg__1"
  tags {
    key: "context__sys_info__name"
    value: "<hostname>"
  }
  tags {
    key: "context__executable_name"
    value: "<path-of-binary>"
  }
}
metrics {
  metricname: "context__cpu_info__load_avg__2"
  tags {
    key: "context__sys_info__name"
    value: "<hostname>"
  }
  tags {
    key: "context__executable_name"
    value: "<path-of-binary>"
  }
}
metrics {
  metricname: "run__iterations"
  tags {
    key: "run__benchmark_name"
    value: "Bench"
  }
  tags {
    key: "context__sys_info__name"
    value: "<hostname>"
  }
  tags {
    key: "context__executable_name"
    value: "<path-of-binary>"
  }
}
metrics {
  metricname: "run__cpu_accumulated_time"
  tags {
    key: "run__benchmark_name"
    value: "Bench"
  }
  tags {
    key: "context__sys_info__name"
    value: "<hostname>"
  }
  tags {
    key: "context__executable_name"
    value: "<path-of-binary>"
  }
}
metrics {
  metricname: "run__real_accumulated_time"
  tags {
    key: "run__benchmark_name"
    value: "Bench"
  }
  tags {
    key: "context__sys_info__name"
    value: "<hostname>"
  }
  tags {
    key: "context__executable_name"
    value: "<path-of-binary>"
  }
}
metrics {
  metricname: "run__adjusted_cpu_time"
  tags {
    key: "run__benchmark_name"
    value: "Bench"
  }
  tags {
    key: "context__sys_info__name"
    value: "<hostname>"
  }
  tags {
    key: "context__executable_name"
    value: "<path-of-binary>"
  }
}
metrics {
  metricname: "run__adjusted_real_time"
  tags {
    key: "run__benchmark_name"
    value: "Bench"
  }
  tags {
    key: "context__sys_info__name"
    value: "<hostname>"
  }
  tags {
    key: "context__executable_name"
    value: "<path-of-binary>"
  }
}
metrics {
  metricname: "run__max_heapbytes_used"
  tags {
    key: "run__benchmark_name"
    value: "Bench"
  }
  tags {
    key: "context__sys_info__name"
    value: "<hostname>"
  }
  tags {
    key: "context__executable_name"
    value: "<path-of-binary>"
  }
}
metrics {
  metricname: "run__allocs_per_iter"
  tags {
    key: "run__benchmark_name"
    value: "Bench"
  }
  tags {
    key: "context__sys_info__name"
    value: "<hostname>"
  }
  tags {
    key: "context__executable_name"
    value: "<path-of-binary>"
  }
}
metrics {
  metricname: "run__memory_result__num_allocs"
  tags {
    key: "run__benchmark_name"
    value: "Bench"
  }
  tags {
    key: "context__sys_info__name"
    value: "<hostname>"
  }
  tags {
    key: "context__executable_name"
    value: "<path-of-binary>"
  }
}
metrics {
  metricname: "run__memory_result__max_bytes_used"
  tags {
    key: "run__benchmark_name"
    value: "Bench"
  }
  tags {
    key: "context__sys_info__name"
    value: "<hostname>"
  }
  tags {
    key: "context__executable_name"
    value: "<path-of-binary>"
  }
}
metrics {
  metricname: "run__memory_result__total_allocated_bytes"
  tags {
    key: "run__benchmark_name"
    value: "Bench"
  }
  tags {
    key: "context__sys_info__name"
    value: "<hostname>"
  }
  tags {
    key: "context__executable_name"
    value: "<path-of-binary>"
  }
}
metrics {
  metricname: "run__memory_result__net_heap_growth"
  tags {
    key: "run__benchmark_name"
    value: "Bench"
  }
  tags {
    key: "context__sys_info__name"
    value: "<hostname>"
  }
  tags {
    key: "context__executable_name"
    value: "<path-of-binary>"
  }
}
metrics {
  metricname: "run__counters__custom-0"
  tags {
    key: "run__benchmark_name"
    value: "Bench"
  }
  tags {
    key: "context__sys_info__name"
    value: "<hostname>"
  }
  tags {
    key: "context__executable_name"
    value: "<path-of-binary>"
  }
  tags {
    key: "unit"
    value: "1000"
  }
  tags {
    key: "flags"
    value: "0"
  }
}
metrics {
  metricname: "run__counters__custom-1"
  tags {
    key: "run__benchmark_name"
    value: "Bench"
  }
  tags {
    key: "context__sys_info__name"
    value: "<hostname>"
  }
  tags {
    key: "context__executable_name"
    value: "<path-of-binary>"
  }
  tags {
    key: "unit"
    value: "1024"
  }
  tags {
    key: "flags"
    value: "1"
  }
}
metrics {
  metricname: "run__counters__custom-2"
  tags {
    key: "run__benchmark_name"
    value: "Bench"
  }
  tags {
    key: "context__sys_info__name"
    value: "<hostname>"
  }
  tags {
    key: "context__executable_name"
    value: "<path-of-binary>"
  }
  tags {
    key: "unit"
    value: "1000"
  }
  tags {
    key: "flags"
    value: "0"
  }
}
metrics {
  metricname: "run__iterations"
  tags {
    key: "run__benchmark_name"
    value: "Error"
  }
  tags {
    key: "run__skipped"
    value: "error"
  }
  tags {
    key: "run__skip_message"
    value: "meanwhile the world goes on"
  }
  tags {
    key: "context__sys_info__name"
    value: "<hostname>"
  }
  tags {
    key: "context__executable_name"
    value: "<path-of-binary>"
  }
}
metrics {
  metricname: "run__cpu_accumulated_time"
  tags {
    key: "run__benchmark_name"
    value: "Error"
  }
  tags {
    key: "run__skipped"
    value: "error"
  }
  tags {
    key: "run__skip_message"
    value: "meanwhile the world goes on"
  }
  tags {
    key: "context__sys_info__name"
    value: "<hostname>"
  }
  tags {
    key: "context__executable_name"
    value: "<path-of-binary>"
  }
}
metrics {
  metricname: "run__real_accumulated_time"
  tags {
    key: "run__benchmark_name"
    value: "Error"
  }
  tags {
    key: "run__skipped"
    value: "error"
  }
  tags {
    key: "run__skip_message"
    value: "meanwhile the world goes on"
  }
  tags {
    key: "context__sys_info__name"
    value: "<hostname>"
  }
  tags {
    key: "context__executable_name"
    value: "<path-of-binary>"
  }
}
metrics {
  metricname: "run__adjusted_cpu_time"
  tags {
    key: "run__benchmark_name"
    value: "Error"
  }
  tags {
    key: "run__skipped"
    value: "error"
  }
  tags {
    key: "run__skip_message"
    value: "meanwhile the world goes on"
  }
  tags {
    key: "context__sys_info__name"
    value: "<hostname>"
  }
  tags {
    key: "context__executable_name"
    value: "<path-of-binary>"
  }
}
metrics {
  metricname: "run__adjusted_real_time"
  tags {
    key: "run__benchmark_name"
    value: "Error"
  }
  tags {
    key: "run__skipped"
    value: "error"
  }
  tags {
    key: "run__skip_message"
    value: "meanwhile the world goes on"
  }
  tags {
    key: "context__sys_info__name"
    value: "<hostname>"
  }
  tags {
    key: "context__executable_name"
    value: "<path-of-binary>"
  }
}
metrics {
  metricname: "run__max_heapbytes_used"
  tags {
    key: "run__benchmark_name"
    value: "Error"
  }
  tags {
    key: "run__skipped"
    value: "error"
  }
  tags {
    key: "run__skip_message"
    value: "meanwhile the world goes on"
  }
  tags {
    key: "context__sys_info__name"
    value: "<hostname>"
  }
  tags {
    key: "context__executable_name"
    value: "<path-of-binary>"
  }
}
metrics {
  metricname: "run__allocs_per_iter"
  tags {
    key: "run__benchmark_name"
    value: "Error"
  }
  tags {
    key: "run__skipped"
    value: "error"
  }
  tags {
    key: "run__skip_message"
    value: "meanwhile the world goes on"
  }
  tags {
    key: "context__sys_info__name"
    value: "<hostname>"
  }
  tags {
    key: "context__executable_name"
    value: "<path-of-binary>"
  }
}
metrics {
  metricname: "run__memory_result__num_allocs"
  tags {
    key: "run__benchmark_name"
    value: "Error"
  }
  tags {
    key: "run__skipped"
    value: "error"
  }
  tags {
    key: "run__skip_message"
    value: "meanwhile the world goes on"
  }
  tags {
    key: "context__sys_info__name"
    value: "<hostname>"
  }
  tags {
    key: "context__executable_name"
    value: "<path-of-binary>"
  }
}
metrics {
  metricname: "run__memory_result__max_bytes_used"
  tags {
    key: "run__benchmark_name"
    value: "Error"
  }
  tags {
    key: "run__skipped"
    value: "error"
  }
  tags {
    key: "run__skip_message"
    value: "meanwhile the world goes on"
  }
  tags {
    key: "context__sys_info__name"
    value: "<hostname>"
  }
  tags {
    key: "context__executable_name"
    value: "<path-of-binary>"
  }
}
metrics {
  metricname: "run__memory_result__total_allocated_bytes"
  tags {
    key: "run__benchmark_name"
    value: "Error"
  }
  tags {
    key: "run__skipped"
    value: "error"
  }
  tags {
    key: "run__skip_message"
    value: "meanwhile the world goes on"
  }
  tags {
    key: "context__sys_info__name"
    value: "<hostname>"
  }
  tags {
    key: "context__executable_name"
    value: "<path-of-binary>"
  }
}
metrics {
  metricname: "run__memory_result__net_heap_growth"
  tags {
    key: "run__benchmark_name"
    value: "Error"
  }
  tags {
    key: "run__skipped"
    value: "error"
  }
  tags {
    key: "run__skip_message"
    value: "meanwhile the world goes on"
  }
  tags {
    key: "context__sys_info__name"
    value: "<hostname>"
  }
  tags {
    key: "context__executable_name"
    value: "<path-of-binary>"
  }
}
metrics {
  metricname: "run__iterations"
  tags {
    key: "run__benchmark_name"
    value: "Message"
  }
  tags {
    key: "run__skipped"
    value: "message"
  }
  tags {
    key: "run__skip_message"
    value: "For the powerful, crimes are those that others commit."
  }
  tags {
    key: "context__sys_info__name"
    value: "<hostname>"
  }
  tags {
    key: "context__executable_name"
    value: "<path-of-binary>"
  }
}
metrics {
  metricname: "run__cpu_accumulated_time"
  tags {
    key: "run__benchmark_name"
    value: "Message"
  }
  tags {
    key: "run__skipped"
    value: "message"
  }
  tags {
    key: "run__skip_message"
    value: "For the powerful, crimes are those that others commit."
  }
  tags {
    key: "context__sys_info__name"
    value: "<hostname>"
  }
  tags {
    key: "context__executable_name"
    value: "<path-of-binary>"
  }
}
metrics {
  metricname: "run__real_accumulated_time"
  tags {
    key: "run__benchmark_name"
    value: "Message"
  }
  tags {
    key: "run__skipped"
    value: "message"
  }
  tags {
    key: "run__skip_message"
    value: "For the powerful, crimes are those that others commit."
  }
  tags {
    key: "context__sys_info__name"
    value: "<hostname>"
  }
  tags {
    key: "context__executable_name"
    value: "<path-of-binary>"
  }
}
metrics {
  metricname: "run__adjusted_cpu_time"
  tags {
    key: "run__benchmark_name"
    value: "Message"
  }
  tags {
    key: "run__skipped"
    value: "message"
  }
  tags {
    key: "run__skip_message"
    value: "For the powerful, crimes are those that others commit."
  }
  tags {
    key: "context__sys_info__name"
    value: "<hostname>"
  }
  tags {
    key: "context__executable_name"
    value: "<path-of-binary>"
  }
}
metrics {
  metricname: "run__adjusted_real_time"
  tags {
    key: "run__benchmark_name"
    value: "Message"
  }
  tags {
    key: "run__skipped"
    value: "message"
  }
  tags {
    key: "run__skip_message"
    value: "For the powerful, crimes are those that others commit."
  }
  tags {
    key: "context__sys_info__name"
    value: "<hostname>"
  }
  tags {
    key: "context__executable_name"
    value: "<path-of-binary>"
  }
}
metrics {
  metricname: "run__max_heapbytes_used"
  tags {
    key: "run__benchmark_name"
    value: "Message"
  }
  tags {
    key: "run__skipped"
    value: "message"
  }
  tags {
    key: "run__skip_message"
    value: "For the powerful, crimes are those that others commit."
  }
  tags {
    key: "context__sys_info__name"
    value: "<hostname>"
  }
  tags {
    key: "context__executable_name"
    value: "<path-of-binary>"
  }
}
metrics {
  metricname: "run__allocs_per_iter"
  tags {
    key: "run__benchmark_name"
    value: "Message"
  }
  tags {
    key: "run__skipped"
    value: "message"
  }
  tags {
    key: "run__skip_message"
    value: "For the powerful, crimes are those that others commit."
  }
  tags {
    key: "context__sys_info__name"
    value: "<hostname>"
  }
  tags {
    key: "context__executable_name"
    value: "<path-of-binary>"
  }
}
metrics {
  metricname: "run__memory_result__num_allocs"
  tags {
    key: "run__benchmark_name"
    value: "Message"
  }
  tags {
    key: "run__skipped"
    value: "message"
  }
  tags {
    key: "run__skip_message"
    value: "For the powerful, crimes are those that others commit."
  }
  tags {
    key: "context__sys_info__name"
    value: "<hostname>"
  }
  tags {
    key: "context__executable_name"
    value: "<path-of-binary>"
  }
}
metrics {
  metricname: "run__memory_result__max_bytes_used"
  tags {
    key: "run__benchmark_name"
    value: "Message"
  }
  tags {
    key: "run__skipped"
    value: "message"
  }
  tags {
    key: "run__skip_message"
    value: "For the powerful, crimes are those that others commit."
  }
  tags {
    key: "context__sys_info__name"
    value: "<hostname>"
  }
  tags {
    key: "context__executable_name"
    value: "<path-of-binary>"
  }
}
metrics {
  metricname: "run__memory_result__total_allocated_bytes"
  tags {
    key: "run__benchmark_name"
    value: "Message"
  }
  tags {
    key: "run__skipped"
    value: "message"
  }
  tags {
    key: "run__skip_message"
    value: "For the powerful, crimes are those that others commit."
  }
  tags {
    key: "context__sys_info__name"
    value: "<hostname>"
  }
  tags {
    key: "context__executable_name"
    value: "<path-of-binary>"
  }
}
metrics {
  metricname: "run__memory_result__net_heap_growth"
  tags {
    key: "run__benchmark_name"
    value: "Message"
  }
  tags {
    key: "run__skipped"
    value: "message"
  }
  tags {
    key: "run__skip_message"
    value: "For the powerful, crimes are those that others commit."
  }
  tags {
    key: "context__sys_info__name"
    value: "<hostname>"
  }
  tags {
    key: "context__executable_name"
    value: "<path-of-binary>"
  }
})";

TEST(Bestie, RunBenchmarkCheckOutput) {
  auto mockrun = [](benchmark::State& state) {
    volatile uint64_t value = 0;
    for (auto _ [[maybe_unused]] : state) {
      value = value + 1;
    }
    state.counters["custom-0"] = 42;
    state.counters["custom-1"] =
        benchmark::Counter(69, benchmark::Counter::kIsRate, benchmark::Counter::OneK::kIs1024);
    state.counters["custom-2"] = 420;
  };
  auto mockerror = [](benchmark::State& state) {
    state.SkipWithError("meanwhile the world goes on");
  };
  auto mockmessage = [](benchmark::State& state) {
    state.SkipWithMessage("For the powerful, crimes are those that others commit.");
  };

  benchmark::RegisterBenchmark("Bench", mockrun);
  benchmark::RegisterBenchmark("Error", mockerror);
  benchmark::RegisterBenchmark("Message", mockmessage);

  bestie::proto::TestMetrics from_call;
  auto capture = [&from_call](const google::protobuf::Message& message, std::ostream& ostream,
                              std::ostream& estream) {
    from_call.CopyFrom(message);
    return bestie::OutputDefault(message, ostream, estream);
  };

  bestie::Reporter reporter(capture);

  std::array flags{
      "<path-of-binary>",
  };
  int size = std::size(flags);

  char hostbuffer[1024] = {};
  ASSERT_FALSE(gethostname(hostbuffer, std::size(hostbuffer)));
  std::string hostname(hostbuffer);

  int64_t start = std::chrono::duration_cast<std::chrono::nanoseconds>(
                      std::chrono::system_clock::now().time_since_epoch())
                      .count();
  benchmark::Initialize(&size, (char**)flags.data());
  benchmark::RunSpecifiedBenchmarks(&reporter);
  benchmark::Shutdown();
  int64_t end = std::chrono::duration_cast<std::chrono::nanoseconds>(
                    std::chrono::system_clock::now().time_since_epoch())
                    .count();

  // Check that OutputDefault created the file, and it matches what was supplied to the call.
  std::fstream input(bestie::MetricsPath(), std::ios::in | std::ios::binary);
  ASSERT_TRUE(input.is_open());
  bestie::proto::TestMetrics from_file;
  ASSERT_TRUE(from_file.ParseFromIstream(&input));
  ASSERT_TRUE(google::protobuf::util::MessageDifferencer::Equals(from_call, from_file));

  // Check the protocol message from a semantic standpoint.
  auto CheckTag = [](bestie::proto::Tag& tag, const char* key, const std::string& value, bool* seen,
                     const char* overwrite = nullptr) {
    if (tag.key() != key) return;

    EXPECT_FALSE(*seen) << "tag " << key << " is duplicated";
    *seen = true;

    EXPECT_EQ(tag.value(), value) << "tag " << key << " value mismatch";
    if (overwrite) tag.set_value(overwrite);
  };

  for (auto& metric : *from_call.mutable_metrics()) {
    EXPECT_GT(metric.timestamp(), start);
    EXPECT_LT(metric.timestamp(), end);

    bool found_executable_name = false;
    bool found_sysinfo_name = false;
    for (auto& tag : *metric.mutable_tags()) {
      CheckTag(tag, "context__executable_name", "<path-of-binary>", &found_executable_name);
      CheckTag(tag, "context__sys_info__name", hostname, &found_sysinfo_name, "<hostname>");
    }

    EXPECT_TRUE(found_executable_name) << "metric: " << metric.metricname();
    EXPECT_TRUE(found_sysinfo_name) << "metric: " << metric.metricname();

    // Not much we can do to validate the metric value itself. Reset to 0 so
    // we can do a simple byte by byte comparison below.
    // EXPECT_NE(metric.value(), 0.0);
    metric.set_timestamp(0);
    metric.set_value(0);
  }

  bestie::proto::TestMetrics expected;
  ASSERT_TRUE(google::protobuf::TextFormat::ParseFromString(kExpectedMessageText, &expected));
  ASSERT_TRUE(google::protobuf::util::MessageDifferencer::Equals(expected, from_call));
  // EqualsProto has not been opensourced by gtest / protocol buffer team.
  //  EXPECT_THAT(from_call, testing::EqualsProto(expected));
}
