{ self }:
{ config, lib, pkgs, ... }:

with lib;

let
  cfg = config.programs.tmux-glance;
in {
  options.programs.tmux-glance = {
    enable = mkEnableOption "tmux-glance ambient background agent & process sentinel";

    package = mkOption {
      type = types.package;
      default = self.packages.${pkgs.system}.default;
      description = "The tmux-glance package to install and wire into tmux.";
    };

    keybindings = {
      glance = mkOption {
        type = types.str;
        default = "g";
        description = "Key to open Glance Mode (server-wide fleet view).";
      };

      hub = mkOption {
        type = types.str;
        default = "b";
        description = "Key to open the Attention Hub.";
      };

      vigil = mkOption {
        type = types.str;
        default = "v";
        description = "Key to toggle Vigil watch on the current pane.";
      };
    };

    popup = {
      width = mkOption {
        type = types.str;
        default = "85%";
        description = "Popup window width.";
      };

      height = mkOption {
        type = types.str;
        default = "75%";
        description = "Popup window height.";
      };
    };
  };

  config = mkIf cfg.enable {
    home.packages = [ cfg.package ];

    programs.tmux.extraConfig = ''
      # ==========================================
      # tmux-glance: ambient sentinel orchestration
      # ==========================================
      bind-key ${cfg.keybindings.glance} display-popup -E -w ${cfg.popup.width} -h ${cfg.popup.height} "${cfg.package}/bin/tmux-glance list-all"
      bind-key ${cfg.keybindings.hub} display-popup -E -w ${cfg.popup.width} -h ${cfg.popup.height} "${cfg.package}/bin/tmux-glance list"
      bind-key ${cfg.keybindings.vigil} run-shell "${cfg.package}/bin/tmux-glance toggle-vigil"

      set-hook -g pane-focus-in "run-shell '${cfg.package}/bin/tmux-glance on-focus #{pane_id}'"
    '';
  };
}
