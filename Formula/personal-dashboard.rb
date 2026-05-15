class PersonalDashboard < Formula
  desc "Phone-first personal dashboard daemon. Tailnet-only on kai-server"
  homepage "https://github.com/coilysiren/personal-dashboard"
  url "ssh://git@github.com/coilysiren/personal-dashboard.git", tag: "v0.2.7", revision: "4dbdded2bfb7db409a4f3d52ee5d9dda707c37c1"
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
