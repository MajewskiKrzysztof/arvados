// Copyright (C) The Arvados Authors. All rights reserved.
//
// SPDX-License-Identifier: AGPL-3.0

import React from "react";
import isImage from 'is-image';
import { WithStyles, withStyles } from '@mui/styles';
import { FileTreeData } from 'components/file-tree/file-tree-data';
import { CollectionFileType } from 'models/collection-file';
import { getInlineFileUrl, sanitizeToken } from "views-components/context-menu/actions/helpers";
import { connect } from "react-redux";
import { RootState } from "store/store";
import { Styles } from '@mui/styles';

interface FileThumbnailProps {
    file: FileTreeData;
}

export const FileThumbnail =
    ({ file }: FileThumbnailProps) =>
        file.type === CollectionFileType.FILE && isImage(file.name)
            ? <ImageFileThumbnail file={file} />
            : null;

type ImageFileThumbnailCssRules = Styles<any, any, 'thumbnail'>;

const imageFileThumbnailStyle = withStyles<ImageFileThumbnailCssRules>(theme => ({
    thumbnail: {
        maxWidth: 250,
        margin: `${theme.spacing(1)} 0`,
    }
}));

interface ImageFileThumbnailProps {
    keepWebServiceUrl: string;
    keepWebInlineServiceUrl: string;
}

const mapStateToProps = ({ auth }: RootState): ImageFileThumbnailProps => ({
    keepWebServiceUrl: auth.config.keepWebServiceUrl,
    keepWebInlineServiceUrl: auth.config.keepWebInlineServiceUrl,
});

const ImageFileThumbnail = connect(mapStateToProps)(imageFileThumbnailStyle(
    ({ classes, file, keepWebServiceUrl, keepWebInlineServiceUrl }: WithStyles<ImageFileThumbnailCssRules> & FileThumbnailProps & ImageFileThumbnailProps) =>
        <img
            className={classes.thumbnail}
            alt={file.name}
            src={sanitizeToken(getInlineFileUrl(file.url, keepWebServiceUrl, keepWebInlineServiceUrl))} />
));
