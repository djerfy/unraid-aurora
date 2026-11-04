package api

import (
	"net/http"

	"aurora/internal/model"
)

func (s *Server) vmHostDevices(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := s.serverContext()
	defer cancel()
	devices, err := s.provider.VMHostDevices(ctx)
	if err != nil {
		writeError(w, http.StatusBadGateway, "vm_host_devices_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, devices)
}

func (s *Server) vmAdvanced(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := s.serverContext()
	defer cancel()
	cfg, err := s.provider.VMAdvanced(ctx, r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "vm_advanced_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, cfg)
}

func (s *Server) updateVMAdvanced(w http.ResponseWriter, r *http.Request) {
	if s.cfg.ReadOnly {
		writeError(w, http.StatusForbidden, "read_only", "server is in read-only mode")
		return
	}
	var cfg model.VMAdvancedConfig
	if !s.decode(w, r, &cfg) {
		return
	}
	ctx, cancel := s.serverContext()
	defer cancel()
	if err := s.provider.UpdateVMAdvanced(ctx, r.PathValue("id"), cfg); err != nil {
		writeError(w, http.StatusBadRequest, "vm_advanced_update_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) cloneVM(w http.ResponseWriter, r *http.Request) {
	if s.cfg.ReadOnly {
		writeError(w, http.StatusForbidden, "read_only", "server is in read-only mode")
		return
	}
	var opts model.VMCloneOptions
	if !s.decode(w, r, &opts) {
		return
	}
	ctx, cancel := s.serverContext()
	defer cancel()
	vm, err := s.provider.CloneVM(ctx, r.PathValue("id"), opts)
	if err != nil {
		writeError(w, http.StatusBadRequest, "vm_clone_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusAccepted, vm)
}

func (s *Server) resizeVMDisk(w http.ResponseWriter, r *http.Request) {
	if s.cfg.ReadOnly {
		writeError(w, http.StatusForbidden, "read_only", "server is in read-only mode")
		return
	}
	var opts model.VMDiskResizeOptions
	if !s.decode(w, r, &opts) {
		return
	}
	ctx, cancel := s.serverContext()
	defer cancel()
	if err := s.provider.ResizeVMDisk(ctx, r.PathValue("id"), opts); err != nil {
		writeError(w, http.StatusBadRequest, "vm_resize_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

type vmUSBRequest struct {
	Vendor  string `json:"vendor"`
	Product string `json:"product"`
}

func (s *Server) attachVMUSB(w http.ResponseWriter, r *http.Request) {
	s.changeVMUSB(w, r, true)
}

func (s *Server) detachVMUSB(w http.ResponseWriter, r *http.Request) {
	s.changeVMUSB(w, r, false)
}

func (s *Server) changeVMUSB(w http.ResponseWriter, r *http.Request, attach bool) {
	if s.cfg.ReadOnly {
		writeError(w, http.StatusForbidden, "read_only", "server is in read-only mode")
		return
	}
	var body vmUSBRequest
	if !s.decode(w, r, &body) {
		return
	}
	ctx, cancel := s.serverContext()
	defer cancel()
	var err error
	if attach {
		err = s.provider.AttachVMUSB(ctx, r.PathValue("id"), body.Vendor, body.Product)
	} else {
		err = s.provider.DetachVMUSB(ctx, r.PathValue("id"), body.Vendor, body.Product)
	}
	if err != nil {
		writeError(w, http.StatusBadRequest, "vm_usb_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}
