package(default_visibility = ["//visibility:public"])

load("@io_bazel_rules_go//go:def.bzl", "go_library", "go_test")

go_library(
    name = "go_default_library",
    srcs = [
        "doc.go",
        "strategy.go",
    ],
    importpath = "k8s.io/kubernetes/pkg/registry/networking/networkpolicy",
    deps = [
        "//pkg/api/legacyscheme:go_default_library",
        "//pkg/apis/networking:go_default_library",
        "//pkg/apis/networking/validation:go_default_library",
        "//staging/src/k8s.io/apimachinery/pkg/runtime:go_default_library",
        "//staging/src/k8s.io/apimachinery/pkg/util/validation/field:go_default_library",
        "//staging/src/k8s.io/apiserver/pkg/storage/names:go_default_library",
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
        "//pkg/registry/networking/networkpolicy/storage:all-srcs",
    ],
    tags = ["automanaged"],
)

go_test(
    name = "go_default_test",
    srcs = ["strategy_test.go"],
    embed = [":go_default_library"],
)
