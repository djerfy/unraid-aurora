const users: Record<string, string> = {
  // Page header
  "System": "Système",
  "Users": "Utilisateurs",
  "Accounts with access to the web interface, SMB shares and NFS exports.":
    "Comptes ayant accès à l'interface web, aux partages SMB et aux exports NFS.",
  "1 account": "1 compte",
  "{count} accounts": "{count} comptes",
  "1 administrator": "1 administrateur",
  "{count} administrators": "{count} administrateurs",
  "Add user": "Ajouter un utilisateur",

  // Stats
  "Accounts": "Comptes",
  "local users": "utilisateurs locaux",
  "Administrators": "Administrateurs",
  "full web UI access": "accès complet à l'interface web",
  "Read-only": "Lecture seule",
  "restricted shares": "partages restreints",
  "Scoped shares": "Partages ciblés",
  "across all accounts": "tous comptes confondus",

  // Account list
  "Local accounts": "Comptes locaux",
  "Managed by Unraid · permissions apply to SMB and NFS":
    "Gérés par Unraid · les permissions s'appliquent à SMB et NFS",
  "Search users…": "Rechercher des utilisateurs…",
  "No users match your search": "Aucun utilisateur ne correspond à votre recherche",
  "Try another name or clear the filter.": "Essayez un autre nom ou effacez le filtre.",
  "Account": "Compte",
  "Role": "Rôle",
  "Share access": "Accès aux partages",
  "Last active": "Dernière activité",
  "Administrator": "Administrateur",
  "No access": "Aucun accès",
  "All shares": "Tous les partages",
  "never": "jamais",
  "Actions for {name}": "Actions pour {name}",
  "View profile": "Voir le profil",
  "Share permissions": "Permissions des partages",

  // Share access matrix
  "Share access matrix": "Matrice d'accès aux partages",
  "Which account can reach which share": "Quel compte peut accéder à quel partage",
  "1 share": "1 partage",
  "{count} shares": "{count} partages",
  "Share": "Partage",

  // Access model
  "Access model": "Modèle d'accès",
  "How permissions are applied": "Comment les permissions sont appliquées",
  "Full web interface access, array control and settings.":
    "Accès complet à l'interface web, au contrôle de l'array et aux paramètres.",
  "Scoped share access without array or system changes.":
    "Accès à des partages ciblés sans modification de l'array ni du système.",
  "Retained accounts that cannot authenticate.": "Comptes conservés qui ne peuvent pas s'authentifier.",
  "Passwords are validated by Unraid; Aurora never stores credentials.":
    "Les mots de passe sont validés par Unraid ; Aurora ne stocke jamais les identifiants.",

  // Drawer
  "Change password": "Changer le mot de passe",
  "Change password · {name}": "Changer le mot de passe · {name}",
  "UID {uid} · member of users": "UID {uid} · membre de users",
  "UID": "UID",
  "Never": "Jamais",
  "Account type": "Type de compte",
  "Local": "Local",
  "This account has no share access.": "Ce compte n'a aucun accès aux partages.",
  "read / write": "lecture / écriture",
  "read only": "lecture seule",
  "Account management": "Gestion du compte",
  "Accounts are created through Unraid's own user manager. Passwords are sent once to emhttpd, never stored by Aurora.":
    "Les comptes sont créés via le gestionnaire d'utilisateurs d'Unraid. Les mots de passe sont transmis une seule fois à emhttpd et ne sont jamais stockés par Aurora.",

  // Create dialog
  "Creates a Unix account, a Samba account and a share access entry, like the official Add User page.":
    "Crée un compte Unix, un compte Samba et une entrée d'accès aux partages, comme la page officielle Add User.",
  "Use a lowercase name starting with a letter or underscore.":
    "Utilisez un nom en minuscules commençant par une lettre ou un souligné.",
  "A password is required.": "Un mot de passe est requis.",
  "Passwords do not match.": "Les mots de passe ne correspondent pas.",
  "Create user": "Créer l'utilisateur",
  "User name": "Nom d'utilisateur",
  "Media account": "Compte multimédia",
  "Password": "Mot de passe",
  "Confirm password": "Confirmer le mot de passe",

  // Edit dialog
  "Edit · {name}": "Modifier · {name}",

  // Password dialog
  "Enter a new password.": "Saisissez un nouveau mot de passe.",
  "New password": "Nouveau mot de passe",

  // Delete dialog
  "Delete {name}?": "Supprimer {name} ?",
  "user": "utilisateur",
  "The account, its Samba credentials and share access entries are removed.":
    "Le compte, ses identifiants Samba et ses entrées d'accès aux partages sont supprimés.",
  "Delete user": "Supprimer l'utilisateur",

  // Toasts
  "User created": "Utilisateur créé",
  "User updated": "Utilisateur mis à jour",
  "Password changed": "Mot de passe modifié",
  "User deleted": "Utilisateur supprimé",
};

export default users;
