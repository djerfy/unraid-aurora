const gpus: Record<string, string> = {
  "Infrastructure": "Infrastructure",
  "GPUs": "GPU",
  "Live utilisation, memory, temperature and power for the accelerators passed through to this server.":
    "Utilisation, mémoire, température et consommation en direct des accélérateurs transmis à ce serveur.",
  "{count} detected": "{count} détecté(s)",
  "{count} compute process": "{count} processus de calcul",
  "{count} compute processes": "{count} processus de calcul",
  "{percent} average load": "{percent} de charge moyenne",
  "No NVIDIA telemetry available": "Aucune télémétrie NVIDIA disponible",
  "Aurora reads GPU data through nvidia-smi. Install the Unraid Nvidia Driver plugin and expose the device to the host to see utilisation, VRAM, temperatures and power.":
    "Aurora lit les données GPU via nvidia-smi. Installez le plugin Unraid Nvidia Driver et exposez le périphérique à l'hôte pour voir l'utilisation, la VRAM, les températures et la consommation.",
  "Devices": "Périphériques",
  "of {size} allocated": "sur {size} alloués",
  "Average load": "Charge moyenne",
  "across all GPUs": "tous GPU confondus",
  "Power draw": "Consommation",
  "combined instant draw": "consommation instantanée combinée",
  "VRAM": "VRAM",
  "GPU {index} · driver {driver} · {uuid}": "GPU {index} · pilote {driver} · {uuid}",
  "GPU": "GPU",
  "utilisation": "utilisation",
  "Power": "Puissance",
  "Fan": "Ventilateur",
  "Clock": "Fréquence",
  "Power limit {limit} · memory clock {clock}": "Limite de puissance {limit} · fréquence mémoire {clock}",
  "Compute processes": "Processus de calcul",
  "No active compute processes.": "Aucun processus de calcul actif.",
  "process": "processus",
  "PID {pid} · {memory}": "PID {pid} · {memory}",
};

export default gpus;
