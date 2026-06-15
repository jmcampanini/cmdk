class Cmdk < Formula
  desc "Keyboard-driven tmux launcher"
  homepage "https://github.com/jmcampanini/cmdk"
  head "https://github.com/jmcampanini/cmdk.git", branch: "main"

  depends_on "go" => :build

  def install
    ldflags = %W[
      -s -w
      -X github.com/jmcampanini/cmdk/cmd.Version=#{version}
    ]
    system "go", "build", "-buildvcs=false", *std_go_args(output: bin/"cmdk", ldflags:)
    generate_completions_from_executable(bin/"cmdk", "completion")
  end

  test do
    assert_match "cmdk version HEAD-", shell_output("#{bin}/cmdk --version")
  end
end
