{
  description = "Glanceable terminal sentinels and background agent orchestration for tmux";

  inputs = {
    nixpkgs.url = "github:nixos/nixpkgs/nixpkgs-unstable";
  };

  outputs =
    { self, nixpkgs }:
    let
      supportedSystems = [
        "aarch64-darwin"
        "x86_64-darwin"
        "x86_64-linux"
        "aarch64-linux"
      ];
      forAllSystems = nixpkgs.lib.genAttrs supportedSystems;
    in
    {
      packages = forAllSystems (
        system:
        let
          pkgs = nixpkgs.legacyPackages.${system};
        in
        rec {
          # Go engine binary (stateful core).
          glance-engine = pkgs.buildGoModule {
            pname = "glance-engine";
            version = "0.7.0";
            src = ./.;

            # No external Go dependencies — stdlib only.
            vendorHash = null;

            # Only build the engine binary; skip the rest.
            subPackages = [ "cmd/glance-engine" ];

            meta = with pkgs.lib; {
              description = "Go engine binary for tmux-glance";
              homepage = "https://github.com/mhutchinson/tmux-glance";
              license = licenses.asl20;
              platforms = platforms.unix;
              mainProgram = "glance-engine";
            };
          };

          # Full tmux-glance package: bash dispatcher + engine + sentinels.
          tmux-glance = pkgs.stdenv.mkDerivation {
            pname = "tmux-glance";
            version = "0.7.0";
            src = ./.;

            nativeBuildInputs = [ pkgs.makeWrapper ];

            # glance-engine is a build-time dep — its binary is copied in.
            buildInputs = [ glance-engine ];

            installPhase = ''
              mkdir -p $out/bin $out/share/tmux-glance

              # Install the Go engine binary.
              cp ${glance-engine}/bin/glance-engine $out/bin/glance-engine

              # Install the thin bash dispatcher and plugin entry point.
              cp bin/tmux-glance $out/bin/tmux-glance
              cp glance.tmux $out/share/tmux-glance/

              chmod +x \
                $out/bin/tmux-glance \
                $out/bin/glance-engine \
                $out/share/tmux-glance/glance.tmux

              # Wrap tmux-glance so it can find glance-engine (same bin dir)
              # and all required POSIX/fzf tools.
              wrapProgram $out/bin/tmux-glance \
                --prefix PATH : ${pkgs.lib.makeBinPath [
                  pkgs.tmux
                  pkgs.fzf
                  pkgs.gnugrep
                  pkgs.gnused
                  pkgs.coreutils
                ]} \
                --prefix PATH : $out/bin
            '';

            meta = with pkgs.lib; {
              description = "Glanceable terminal sentinels and background agent orchestration for tmux";
              homepage = "https://github.com/mhutchinson/tmux-glance";
              license = licenses.asl20;
              platforms = platforms.unix;
              mainProgram = "tmux-glance";
            };
          };

          default = tmux-glance;
        }
      );

      checks = forAllSystems (
        system:
        let
          pkgs = nixpkgs.legacyPackages.${system};
          enginePkg = self.packages.${system}.glance-engine;
        in
        {
          # Go unit tests (race detector on Linux; skipped on Darwin Nix sandbox
          # due to missing Mach APIs required by the race detector).
          go-tests = pkgs.stdenv.mkDerivation {
            name = "tmux-glance-go-tests";
            src = ./.;
            nativeBuildInputs = [ pkgs.go ];
            buildPhase = ''
              export HOME=$TMPDIR
              export GOPATH=$TMPDIR/go
              export GOCACHE=$TMPDIR/cache
              # Run with -race on Linux; without on Darwin (Nix sandbox).
              ${if pkgs.stdenv.isLinux then ''
                go test -race ./...
              '' else ''
                go test ./...
              ''}
            '';
            installPhase = "mkdir -p $out && echo ok > $out/result";
          };

          # Integration tests: shell tests run against a real tmux server.
          unit-tests = pkgs.runCommand "tmux-glance-tests" {
            nativeBuildInputs = [
              pkgs.bash
              pkgs.tmux
              pkgs.fzf
              pkgs.gnugrep
              pkgs.gnused
              pkgs.coreutils
              enginePkg  # glance-engine available in PATH for integration tests
            ];
          } ''
            export HOME=$TMPDIR
            mkdir -p $out test-build
            cp -r ${./.}/* test-build/
            cd test-build
            chmod -R u+w .
            # Install the built engine binary so bin/tmux-glance can find it.
            cp ${enginePkg}/bin/glance-engine bin/glance-engine
            chmod +x bin/* tests/*
            patchShebangs .
            bash tests/run_tests.sh | tee $out/test.log
          '';
        }
      );

      homeManagerModules = {
        tmux-glance = import ./nix/home-manager.nix { inherit self; };
        default = self.homeManagerModules.tmux-glance;
      };
    };
}
