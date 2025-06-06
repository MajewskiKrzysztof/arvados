/*
 * Copyright (C) The Arvados Authors. All rights reserved.
 *
 * SPDX-License-Identifier: AGPL-3.0 OR Apache-2.0
 *
 */

package org.arvados.client.logic.keep;

import com.google.common.collect.Lists;
import org.arvados.client.api.client.CollectionsApiClient;
import org.arvados.client.api.client.KeepWebApiClient;
import org.arvados.client.api.model.Collection;
import org.arvados.client.common.Characters;
import org.arvados.client.exception.ArvadosClientException;
import org.arvados.client.logic.collection.FileToken;
import org.arvados.client.logic.collection.ManifestDecoder;
import org.arvados.client.logic.collection.ManifestStream;
import org.arvados.client.logic.keep.exception.DownloadFolderAlreadyExistsException;
import org.arvados.client.logic.keep.exception.FileAlreadyExistsException;
import org.slf4j.Logger;

import java.io.File;
import java.io.FileOutputStream;
import java.io.IOException;
import java.io.InputStream;
import java.io.RandomAccessFile;
import java.nio.file.Files;
import java.util.ArrayList;
import java.util.List;
import java.util.concurrent.CompletableFuture;
import java.util.stream.Collectors;
import java.util.stream.Stream;

public class FileDownloader {

    private final ManifestDecoder manifestDecoder;
    private final CollectionsApiClient collectionsApiClient;
    private final KeepWebApiClient keepWebApiClient;
    private final Logger log = org.slf4j.LoggerFactory.getLogger(FileDownloader.class);

    public FileDownloader(ManifestDecoder manifestDecoder, CollectionsApiClient collectionsApiClient, KeepWebApiClient keepWebApiClient) {
        this.manifestDecoder = manifestDecoder;
        this.collectionsApiClient = collectionsApiClient;
        this.keepWebApiClient = keepWebApiClient;
    }

    public List<FileToken> listFileInfoFromCollection(String collectionUuid) {
        Collection requestedCollection = collectionsApiClient.get(collectionUuid);
        String manifestText = requestedCollection.getManifestText();

        // decode manifest text and get list of all FileTokens for this collection
        return manifestDecoder.decode(manifestText)
                .stream()
                .flatMap(p -> p.getFileTokens().stream())
                .collect(Collectors.toList());
    }

    public File downloadSingleFileUsingKeepWeb(String filePathName, String collectionUuid, String pathToDownloadFolder) {
        FileToken fileToken = getFileTokenFromCollection(filePathName, collectionUuid);
        if (fileToken == null) {
            throw new ArvadosClientException(String.format("%s not found in Collection with UUID %s", filePathName, collectionUuid));
        }

        File downloadedFile = checkIfFileExistsInTargetLocation(fileToken, pathToDownloadFolder);
        try (FileOutputStream fos = new FileOutputStream(downloadedFile)) {
            fos.write(keepWebApiClient.download(collectionUuid, filePathName));
        } catch (IOException e) {
            throw new ArvadosClientException(String.format("Unable to write down file %s", fileToken.getFileName()), e);
        }
        return downloadedFile;
    }

    public File downloadFileWithResume(String collectionUuid, String fileName, String pathToDownloadFolder, long start, Long end) throws IOException {
        if (end != null && end < start) {
            throw new IllegalArgumentException("End index must be greater than or equal to the start index");
        }

        File destinationFile = new File(pathToDownloadFolder, fileName);

        if (!destinationFile.exists()) {
            boolean isCreated = destinationFile.createNewFile();
            if (!isCreated) {
                throw new IOException("Failed to create new file: " + destinationFile.getAbsolutePath());
            }
        }

        try (RandomAccessFile outputFile = new RandomAccessFile(destinationFile, "rw");
             InputStream inputStream = keepWebApiClient.get(collectionUuid, fileName, start, end)) {
            outputFile.seek(start);

            long remaining = (end == null) ? Long.MAX_VALUE : end - start + 1;
            byte[] buffer = new byte[4096];
            int bytesRead;
            while ((bytesRead = inputStream.read(buffer)) != -1 && remaining > 0) {
                int bytesToWrite = (int) Math.min(bytesRead, remaining);
                outputFile.write(buffer, 0, bytesToWrite);
                remaining -= bytesToWrite;
            }
        }

        return destinationFile;
    }

    public List<File> downloadFilesFromCollectionUsingKeepWeb(String collectionUuid, String pathToDownloadFolder) {
        String collectionTargetDir = setTargetDirectory(collectionUuid, pathToDownloadFolder).getAbsolutePath();
        List<FileToken> fileTokens = listFileInfoFromCollection(collectionUuid);

        List<CompletableFuture<File>> futures = Lists.newArrayList();
        for (FileToken fileToken : fileTokens) {
            futures.add(CompletableFuture.supplyAsync(() -> this.downloadOneFileFromCollectionUsingKeepWeb(fileToken, collectionUuid, collectionTargetDir)));
        }

        @SuppressWarnings("unchecked")
        CompletableFuture<File>[] array = futures.toArray(new CompletableFuture[0]);
        return Stream.of(array)
                .map(CompletableFuture::join).collect(Collectors.toList());
    }

    private FileToken getFileTokenFromCollection(String filePathName, String collectionUuid) {
        return listFileInfoFromCollection(collectionUuid)
                .stream()
                .filter(p -> (p.getFullPath()).equals(filePathName))
                .findFirst()
                .orElse(null);
    }

    private File checkIfFileExistsInTargetLocation(FileToken fileToken, String pathToDownloadFolder) {
        String fileName = fileToken.getFileName();

        File downloadFile = new File(pathToDownloadFolder + Characters.SLASH + fileName);
        if (downloadFile.exists()) {
            throw new FileAlreadyExistsException(String.format("File %s exists in location %s", fileName, pathToDownloadFolder));
        } else {
            return downloadFile;
        }
    }

    private File downloadOneFileFromCollectionUsingKeepWeb(FileToken fileToken, String collectionUuid, String pathToDownloadFolder) {
        String filePathName = fileToken.getPath() + fileToken.getFileName();
        File downloadedFile = new File(pathToDownloadFolder + Characters.SLASH + filePathName);
        downloadedFile.getParentFile().mkdirs();

        try (FileOutputStream fos = new FileOutputStream(downloadedFile)) {
            fos.write(keepWebApiClient.download(collectionUuid, filePathName));
        } catch (IOException e) {
            throw new RuntimeException(e);
        }
        return downloadedFile;
    }

    public List<File> downloadFilesFromCollection(String collectionUuid, String pathToDownloadFolder) {

        // download requested collection and extract manifest text
        Collection requestedCollection = collectionsApiClient.get(collectionUuid);
        String manifestText = requestedCollection.getManifestText();

        // if directory with this collectionUUID does not exist - create one
        // if exists - abort (throw exception)
        File collectionTargetDir = setTargetDirectory(collectionUuid, pathToDownloadFolder);

        // decode manifest text and create list of ManifestStream objects containing KeepLocators and FileTokens
        List<ManifestStream> manifestStreams = manifestDecoder.decode(manifestText);

        //list of all downloaded files that will be returned by this method
        List<File> downloadedFilesFromCollection = new ArrayList<>();

        // download files for each manifest stream
        for (ManifestStream manifestStream : manifestStreams)
            downloadedFilesFromCollection.addAll(downloadFilesFromSingleManifestStream(collectionUuid, manifestStream, collectionTargetDir));

        log.debug(String.format("Total of: %d files downloaded", downloadedFilesFromCollection.size()));
        return downloadedFilesFromCollection;
    }

    private File setTargetDirectory(String collectionUUID, String pathToDownloadFolder) {
        //local directory to save downloaded files
        File collectionTargetDir = new File(pathToDownloadFolder + Characters.SLASH + collectionUUID);
        if (collectionTargetDir.exists()) {
            throw new DownloadFolderAlreadyExistsException(String.format("Directory for collection UUID %s already exists", collectionUUID));
        } else {
            collectionTargetDir.mkdirs();
        }
        return collectionTargetDir;
    }

    private List<File> downloadFilesFromSingleManifestStream(String collectionUuid, ManifestStream manifestStream, File collectionTargetDir){
        List<File> downloadedFiles = new ArrayList<>();

        for (FileToken fileToken : manifestStream.getFileTokens()) {
            File downloadedFile = new File(collectionTargetDir.getAbsolutePath() + Characters.SLASH + fileToken.getFullPath()); //create file
            downloadedFile.getParentFile().mkdirs();

            try  {
                byte[] download = keepWebApiClient.download(collectionUuid, fileToken.getFileName());
                Files.write(downloadedFile.toPath(), download);
            } catch (IOException | ArvadosClientException e) {
                throw new ArvadosClientException(String.format("Unable to write down file %s", fileToken.getFileName()), e);
            }

            downloadedFiles.add(downloadedFile);
            log.debug(String.format("File %d / %d downloaded from manifest stream",
                    manifestStream.getFileTokens().indexOf(fileToken) + 1,
                    manifestStream.getFileTokens().size()));
        }
        return downloadedFiles;
    }

}