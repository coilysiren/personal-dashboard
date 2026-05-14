class PersonalDashboard < Formula
  desc "Phone-first personal dashboard daemon. Tailnet-only on kai-server"
  homepage "https://github.com/coilysiren/personal-dashboard"
  url "https://github.com/coilysiren/personal-dashboard/archive/refs/tags/v0.2.0.tar.gz"
  sha256 "739b482cd631a5bbe9ee86d143d9739181f446c01194a9f217cbf3b8bee99d26"
  license "MIT"
  head "https://github.com/coilysiren/personal-dashboard.git", branch: "main"

  depends_on "go" => :build

  def install
    ldflags = "-s -w -X main.Version=v#{version}"
    system "go", "build", "-trimpath",
           "-ldflags", ldflags,
           "-o", bin/"personal-dashboard",
           "./cmd/personal-dashboard"
  end

  # No `service do` block on purpose. On kai-server this runs under
  # systemd via infrastructure/scripts/install-personal-dashboard.sh, not
  # under `brew services`. On a laptop it would only be useful for
  # development and `make run` covers that.
  def caveats
    <<~EOS
      personal-dashboard is intended to run under systemd on kai-server.
      See coilysiren/personal-dashboard/docs/deploy.md for the runbook,
      or coilysiren/infrastructure/scripts/install-personal-dashboard.sh
      for the one-shot install.
    EOS
  end

  test do
    assert_match "personal-dashboard", shell_output("#{bin}/personal-dashboard --help 2>&1", 2)
  end
end
