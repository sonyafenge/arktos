load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "go_default_library",
    srcs = [
        "factory.go",
        "interface.go",
        "manager.go",
        "measurement_executor.go",
        "summary.go",
    ],
    importpath = "k8s.io/kubernetes/perf-tests/clusterloader2/pkg/measurement",
    visibility = ["//visibility:public"],
    deps = [
        "//perf-tests/clusterloader2/api:go_default_library",
        "//perf-tests/clusterloader2/pkg/config:go_default_library",
        "//perf-tests/clusterloader2/pkg/errors:go_default_library",
        "//perf-tests/clusterloader2/pkg/framework:go_default_library",
        "//staging/src/k8s.io/apimachinery/pkg/util/wait:go_default_library",
    ],
)

filegroup(
    name = "package-srcs",
    srcs = glob(["**"]),
    tags = ["automanaged"],
    visibility = ["//visibility:private"],
)

filegroup(
    name = "all-srcs",
    srcs = [
        ":package-srcs",
        "//perf-tests/clusterloader2/pkg/measurement/common:all-srcs",
        "//perf-tests/clusterloader2/pkg/measurement/util:all-srcs",
    ],
    tags = ["automanaged"],
    visibility = ["//visibility:public"],
)
