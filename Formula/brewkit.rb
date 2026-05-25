class Brewkit < Formula
  desc "Manage Homebrew packages across layered profiles"
  homepage "https://github.com/jmcampanini/brewkit"
  head "https://github.com/jmcampanini/brewkit.git", branch: "main"

  depends_on "go" => :build

  def install
    ldflags = "-s -w -X github.com/jmcampanini/brewkit/internal/cli.Version=HEAD-#{Utils.git_short_head}"
    system "go", "build", *std_go_args(output: bin/"brewkit", ldflags: ldflags), "./cmd/brewkit"
  end

  test do
    assert_match "brewkit version HEAD-", shell_output("#{bin}/brewkit --version")
  end
end
