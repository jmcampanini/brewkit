class Brewkit < Formula
  desc "Manage Homebrew packages across layered profiles"
  homepage "https://github.com/jmcampanini/brewkit"
  head "https://github.com/jmcampanini/brewkit.git", branch: "main"

  depends_on "go" => :build

  def install
    ldflags = %W[
      -s -w
      -X github.com/jmcampanini/brewkit/internal/cli.Version=#{version}
    ]
    system "go", "build", "-buildvcs=false", *std_go_args(output: bin/"brewkit", ldflags:), "./cmd/brewkit"
    generate_completions_from_executable(bin/"brewkit", "completion")
  end

  test do
    assert_match "brewkit version HEAD-", shell_output("#{bin}/brewkit --version")
  end
end
