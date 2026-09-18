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
          tmux-glance = pkgs.stdenv.mkDerivation {
            pname = "tmux-glance";
            version = "0.1.0";
            src = ./.;

            nativeBuildInputs = [ pkgs.makeWrapper ];

            installPhase = ''
              mkdir -p $out/bin $out/share/tmux-glance/sentinels
              cp bin/tmux-glance $out/bin/tmux-glance
              cp sentinels/*.sh $out/share/tmux-glance/sentinels/
              cp glance.tmux $out/share/tmux-glance/

              chmod +x $out/bin/tmux-glance $out/share/tmux-glance/sentinels/*.sh $out/share/tmux-glance/glance.tmux

              wrapProgram $out/bin/tmux-glance \
                --prefix PATH : ${pkgs.lib.makeBinPath [ pkgs.tmux pkgs.fzf pkgs.gnugrep pkgs.gnused pkgs.coreutils ]} \
                --set TMUX_GLANCE_SENTINEL_DIR "$out/share/tmux-glance/sentinels"
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
        in
        {
          unit-tests = pkgs.runCommand "tmux-glance-tests" {
            nativeBuildInputs = [
              pkgs.bash
              pkgs.tmux
              pkgs.fzf
              pkgs.gnugrep
              pkgs.gnused
              pkgs.coreutils
            ];
          } ''
            export HOME=$TMPDIR
            mkdir -p $out test-build
            cp -r ${./.}/* test-build/
            cd test-build
            chmod -R u+w .
            chmod +x bin/* sentinels/* tests/*
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
