const balance: Record<string, string> = {
  // Page header
  "Infrastructure": "Infrastructure",
  "Balance": "Équilibrage",
  "Empty a disk or move folders to other disks in the background, with progress and per-file details.":
    "Videz un disque ou déplacez des dossiers vers d'autres disques en arrière-plan, avec progression et détail fichier par fichier.",
  "1 disk mounted": "1 disque monté",
  "{count} disks mounted": "{count} disques montés",
  "Dry run": "Simulation",

  // Job states and phases
  "Idle": "Inactif",
  "Scanning": "Analyse",
  "Cancelling": "Annulation",
  "Completed": "Terminé",
  "Copying": "Copie",
  "Cleaning": "Nettoyage",

  // Operation form
  "New operation": "Nouvelle opération",
  "source → destination(s)": "source → destination(s)",
  "Source disk": "Disque source",
  "Data": "Données",
  "Cache": "Cache",
  "Pool": "Pool",
  "{used} used · {free} free": "{used} utilisés · {free} libres",
  "Destination(s)": "Destination(s)",
  "{free} free": "{free} libres",
  "Total space available:": "Espace cumulé disponible :",
  "Empty disk": "Vider le disque",
  "Folders / files": "Dossiers / fichiers",
  "Delete source": "Supprimer la source",
  "Overwrite": "Écraser",
  "disk": "disque",

  // File browser
  "Reading disk…": "Lecture du disque…",
  "Empty folder.": "Dossier vide.",
  "Deselect {name}": "Désélectionner {name}",
  "Select {name}": "Sélectionner {name}",
  "Remove {path}": "Retirer {path}",
  "All content on the source disk will be moved, except": "Tout le contenu du disque source sera déplacé, sauf",
  "and hidden folders.": "et les dossiers cachés.",

  // History
  "History": "Historique",
  "1 run": "1 exécution",
  "{count} runs": "{count} exécutions",
  "No balance run yet": "Aucun équilibrage",
  "Completed operations will appear here with file details.":
    "Les opérations terminées apparaîtront ici avec le détail des fichiers.",
  "1 file": "1 fichier",
  "{count} files": "{count} fichiers",

  // Confirmation
  "Start balance?": "Lancer l'équilibrage ?",
  "Dry run mode: no writes, no deletions.": "Mode simulation : aucune écriture, aucune suppression.",
  "Files are copied and verified before the source is deleted.":
    "Les fichiers sont copiés puis vérifiés avant suppression de la source.",
  "Start dry run": "Lancer la simulation",
  "Destinations": "Destinations",
  "Scope": "Périmètre",
  "Whole disk": "Tout le disque",
  "1 selection": "1 sélection",
  "{count} selections": "{count} sélections",
  "Check the destinations: existing files are skipped unless overwrite is enabled.":
    "Vérifiez les destinations : les fichiers existants sont ignorés sauf si l'écrasement est activé.",

  // Job details
  "Balance · {id}": "Équilibrage · {id}",
  "Progress": "Progression",
  "Bytes": "Octets",
  "Files": "Fichiers",
  "Speed": "Vitesse",
  "ETA": "Temps restant",
  "Updated": "Mis à jour",
  "Mode": "Mode",
  "Selection": "Sélection",
  "Errors": "Erreurs",
  "{count} entries": "{count} entrées",
  "log ({count} lines)": "journal ({count} lignes)",

  // Toasts
  "Balance job started": "Équilibrage démarré",
  "Dry run: nothing will be written.": "Simulation : aucune écriture.",
  "Running in the background.": "Exécution en arrière-plan.",
  "Unable to start balance": "Impossible de démarrer l'équilibrage",
  "Cancellation requested": "Annulation demandée",
  "Cancel failed": "Échec de l'annulation",
};

export default balance;
