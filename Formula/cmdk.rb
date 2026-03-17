class Cmdk < Formula
  desc "Keyboard-driven tmux launcher"
  homepage "https://github.com/jmcampanini/cmdk"
  head "https://github.com/jmcampanini/cmdk.git", branch: "main"

  depends_on "go" => :build

  def install
    ldflags = "-s -w -X github.com/jmcampanini/cmdk/cmd.Version=HEAD-#{Utils.git_short_head}"
    system "go", "build", *std_go_args(ldflags: ldflags)
  end

  test do
    assert_match "cmdk version HEAD-", shell_output("#{bin}/cmdk --version")
  end
end
