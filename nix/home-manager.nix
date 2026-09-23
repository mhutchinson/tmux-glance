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
        description = "Key to open Glance Mode (all panes cross-session teleporter).";
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

      history = mkOption {
        type = types.str;
        default = "Tab";
        description = "Key to open Jump History Timeline (Tier 2).";
      };

      quickfixNext = mkOption {
        type = types.str;
        default = "]";
        description = "Key to cycle to next attention item (Tier 2).";
      };

      quickfixPrev = mkOption {
        type = types.str;
        default = "[";
        description = "Key to cycle to previous attention item (Tier 2).";
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

    disabledSentinels = mkOption {
      type = types.listOf types.str;
      default = [ ];
      description = "List of sentinel names to disable completely (e.g. [ \"claude\" ]).";
    };

    routes = mkOption {
      type = types.attrsOf types.str;
      default = { };
      description = "Command-to-sentinel routing overrides (e.g. { chat = \"generic\"; my-bot = \"antigravity\"; }).";
    };

    enableHarpoon = mkOption {
      type = types.bool;
      default = false;
      description = "Enable Tier 2 Harpoon fast-jump keybindings (prefix C-h, C-j, C-k, C-l).";
    };

    enableJumplist = mkOption {
      type = types.bool;
      default = false;
      description = "Enable Tier 2 Jumplist backtrack and history keybindings (prefix <, >, Tab, C-z, C-y, and repeatable prefix -r u, U).";
    };

    enableQuickfix = mkOption {
      type = types.bool;
      default = false;
      description = "Enable Tier 2 Quickfix attention cycling keybindings (prefix -r ], [).";
    };

    scanCooldown = mkOption {
      type = types.int;
      default = 3;
      description = "Cooldown threshold in seconds to debounce background scans when focus is unchanged.";
    };
  };

  config = mkIf cfg.enable {
    home.packages = [ cfg.package ];

    programs.tmux.extraConfig = ''
      # ==========================================
      # tmux-glance: ambient sentinel orchestration
      # ==========================================
      ${optionalString (cfg.disabledSentinels != [ ]) ''
        set -g @glance_disabled_sentinels '${concatStringsSep "," cfg.disabledSentinels}'
      ''}
      ${optionalString (cfg.routes != { }) ''
        set -g @glance_routes '${concatStringsSep "," (mapAttrsToList (k: v: "${k}=${v}") cfg.routes)}'
      ''}
      ${optionalString (cfg.scanCooldown != 3) ''
        set -g @glance_scan_cooldown ${toString cfg.scanCooldown}
      ''}
      bind-key ${cfg.keybindings.glance} display-popup -E -w ${cfg.popup.width} -h ${cfg.popup.height} "${cfg.package}/bin/tmux-glance list-all #{pane_id}"
      bind-key ${cfg.keybindings.hub} display-popup -E -w ${cfg.popup.width} -h ${cfg.popup.height} "${cfg.package}/bin/tmux-glance list auto #{pane_id}"
      bind-key ${cfg.keybindings.vigil} run-shell "${cfg.package}/bin/tmux-glance toggle-vigil"

      set-hook -g pane-focus-in "run-shell '${cfg.package}/bin/tmux-glance on-focus #{pane_id}'"
      ${optionalString cfg.enableHarpoon ''
        bind-key -r C-h run-shell "${cfg.package}/bin/tmux-glance jump-slot h #{pane_id}"
        bind-key -r C-j run-shell "${cfg.package}/bin/tmux-glance jump-slot j #{pane_id}"
        bind-key -r C-k run-shell "${cfg.package}/bin/tmux-glance jump-slot k #{pane_id}"
        bind-key -r C-l run-shell "${cfg.package}/bin/tmux-glance jump-slot l #{pane_id}"
      ''}
      ${optionalString cfg.enableJumplist ''
        bind-key ${cfg.keybindings.history} display-popup -E -w ${cfg.popup.width} -h ${cfg.popup.height} "${cfg.package}/bin/tmux-glance list-history #{pane_id}"
        bind-key -r '<' run-shell "${cfg.package}/bin/tmux-glance jump-back #{pane_id}"
        bind-key -r '>' run-shell "${cfg.package}/bin/tmux-glance jump-forward #{pane_id}"
        bind-key C-z run-shell "${cfg.package}/bin/tmux-glance jump-back #{pane_id}"
        bind-key C-y run-shell "${cfg.package}/bin/tmux-glance jump-forward #{pane_id}"
        bind-key -r u run-shell "${cfg.package}/bin/tmux-glance jump-back #{pane_id}"
        bind-key -r U run-shell "${cfg.package}/bin/tmux-glance jump-forward #{pane_id}"
      ''}
      ${optionalString cfg.enableQuickfix ''
        bind-key -r '${cfg.keybindings.quickfixNext}' run-shell "${cfg.package}/bin/tmux-glance next-attention #{pane_id}"
        bind-key -r '${cfg.keybindings.quickfixPrev}' run-shell "${cfg.package}/bin/tmux-glance prev-attention #{pane_id}"
      ''}
    '';
  };
}
