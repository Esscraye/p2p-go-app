# p2p-go-app

## Description

`p2p-go-app` est une application de transfert de fichiers peer-to-peer (P2P) écrite en Go. Elle permet aux utilisateurs de partager et de télécharger des fichiers en divisant les fichiers en parties et en les stockant sur différents pairs. L'application utilise un serveur pour gérer l'enregistrement des pairs et la requête des parties de fichiers.

## Fonctionnalités

- Enregistrement des pairs auprès d'un serveur.
- Téléchargement de parties de fichiers à partir de pairs.
- Division de fichiers en parties pour un partage efficace.
- Combinaison de parties de fichiers en un fichier complet.
- Mise à jour des parties de fichiers sur le serveur.

## Prérequis

- Go (version 1.16 ou supérieure)
- Une connexion Internet pour obtenir l'adresse IP publique.

## Installation

1. Clonez le dépôt :

   ```bash
   git clone https://github.com/Esscraye/p2p-go-app.git
   cd p2p-go-app
   ```

2. Installez les dépendances :

   ```bash
   go mod tidy
   ```

3. Créez un fichier `.env` à la racine du projet pour définir le chemin du fichier journal :

   ```plaintext
   LOG_FILE_PATH=logs/app.log
   ```

## Utilisation

### Démarrer le serveur

Pour démarrer le serveur, exécutez la commande suivante :

```bash
go run main.go 8080
```

### Démarrer un pair

Pour démarrer un pair, exécutez la commande suivante dans un autre terminal :

```bash
go run main.go <port>
```

Remplacez `<port>` par le port que vous souhaitez utiliser (doit être entre 1024 et 65535 et différent de 8080).

### Commandes disponibles

Une fois le pair démarré, vous pouvez entrer un numéro de commande pour effectuer l'une des actions suivantes :

1. Obtenir la liste des pairs.
2. Télécharger une partie de fichier.
3. Télécharger un fichier complet.
4. Mettre à jour les parties de fichiers sur le serveur.
5. Interroger les parties de fichiers.
6. Quitter l'application.
7. Diviser un fichier en parties.
8. Combiner des parties de fichiers en un fichier complet.

## Exemples d'utilisation

- Pour diviser un fichier en parties :

  ```bash
  go run main.go <port>
  ```

  Sélectionnez l'option 7 et entrez le chemin du fichier à diviser.

- Pour combiner des parties de fichiers :
  Sélectionnez l'option 8 et entrez le nom du fichier et le chemin de sortie.
  