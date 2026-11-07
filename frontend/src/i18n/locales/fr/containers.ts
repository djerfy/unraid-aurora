const containers: Record<string, string> = {
  "Container created": "Conteneur créé",
  "{name} was created and started.": "{name} a été créé et démarré.",
  "Creation failed": "Échec de la création",
  "New container": "Nouveau conteneur",
  "Pull an image and create a container, mirroring the main options of the Unraid Add Container page.":
    "Téléchargez une image et créez un conteneur, avec les principales options de la page Add Container d'Unraid.",
  "Create & start": "Créer et démarrer",
  "Template": "Modèle",
  "Load an existing Unraid XML to recreate a container":
    "Chargez un fichier XML Unraid existant pour recréer un conteneur",
  "Container template": "Modèle de conteneur",
  "Start from scratch": "Partir de zéro",
  "Reload": "Recharger",
  "Templates are read from the flash drive; loading one prefills every field and can be used to relaunch a container after a failed creation.":
    "Les modèles sont lus depuis la clé USB ; en charger un préremplit tous les champs et permet de relancer un conteneur après une création échouée.",
  "Container": "Conteneur",
  "Identity, image and network": "Identité, image et réseau",
  "Name": "Nom",
  "Container name": "Nom du conteneur",
  "Repository": "Dépôt",
  "Restart policy": "Politique de redémarrage",
  "Icon URL": "URL de l'icône",
  "Web UI": "Interface web",
  "Privileged mode": "Mode privilégié",
  "Privileged": "Privilégié",
  "Full host access.": "Accès complet à l'hôte.",
  "Extra parameters": "Paramètres supplémentaires",
  "Post arguments": "Arguments postérieurs",
  "Devices, capabilities, resource limits and raw Docker options. Empty fields keep Docker defaults.":
    "Périphériques, capacités, limites de ressources et options Docker brutes. Les champs vides conservent les valeurs par défaut de Docker.",
  "Extra parameters are passed to the Docker create request for supported flags and kept in the Unraid template. Post arguments are appended to the image command.":
    "Les paramètres supplémentaires sont transmis à la requête de création Docker pour les options prises en charge et conservés dans le template Unraid. Les arguments postérieurs sont ajoutés à la commande de l'image.",
  "Grant the container full host device access.":
    "Accorder au conteneur un accès complet aux périphériques de l'hôte.",
  "Paths": "Chemins",
  "Bind mounts, host path to container path": "Montages, chemin de l'hôte vers le chemin du conteneur",
  "Add path": "Ajouter un chemin",
  "No bind mounts configured.": "Aucun montage configuré.",
  "Host path": "Chemin de l'hôte",
  "Container path": "Chemin du conteneur",
  "Read only": "Lecture seule",
  "Remove path": "Retirer le chemin",
  "Ports": "Ports",
  "Published host ports": "Ports hôtes publiés",
  "Add port": "Ajouter un port",
  "No published ports.": "Aucun port publié.",
  "Host port": "Port hôte",
  "Container port": "Port du conteneur",
  "Protocol": "Protocole",
  "Remove port": "Retirer le port",
  "Environment": "Environnement",
  "Variables passed to the container": "Variables transmises au conteneur",
  "Add variable": "Ajouter une variable",
  "No environment variables.": "Aucune variable d'environnement.",
  "Variable name": "Nom de la variable",
  "Variable value": "Valeur de la variable",
  "Remove variable": "Retirer la variable",
  "A container name is required.": "Un nom de conteneur est requis.",
  "The name can only contain letters, digits, '_', '.' and '-'.":
    "Le nom ne peut contenir que des lettres, des chiffres, '_', '.' et '-'.",
  "An image repository is required, for example linuxserver/plex:latest.":
    "Un dépôt d'image est requis, par exemple linuxserver/plex:latest.",
  "Every path needs both a host source and a container destination.":
    "Chaque chemin doit avoir une source sur l'hôte et une destination dans le conteneur.",
  "Container path “{path}” must be absolute.": "Le chemin du conteneur « {path} » doit être absolu.",
  "Ports must be integers between 1 and 65535.":
    "Les ports doivent être des entiers compris entre 1 et 65535.",
  "Environment variable “{name}” has an invalid name.":
    "La variable d'environnement « {name} » a un nom invalide.",
  "Container updated": "Conteneur mis à jour",
  "{name} was recreated with the new settings.": "{name} a été recréé avec les nouveaux paramètres.",
  "Update failed": "Échec de la mise à jour",
  "Settings · {name}": "Paramètres · {name}",
  "Recreate this container with new paths, ports, environment variables and restart policy.":
    "Recréez ce conteneur avec de nouveaux chemins, ports, variables d'environnement et une nouvelle politique de redémarrage.",
  "Apply & recreate": "Appliquer et recréer",
  "Applying changes stops and recreates the container. Its current ID and uptime reset. Only the paths listed below are kept; other bind mounts are removed.":
    "L'application des modifications arrête et recrée le conteneur. Son ID actuel et sa durée de fonctionnement sont réinitialisés. Seuls les chemins listés ci-dessous sont conservés ; les autres montages sont supprimés.",
  "Docker restart behaviour": "Comportement de redémarrage Docker",
};

export default containers;
