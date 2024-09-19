// Copyright (C) The Arvados Authors. All rights reserved.
//
// SPDX-License-Identifier: AGPL-3.0

import React from 'react';
import { CustomStyleRulesCallback } from 'common/custom-theme';
import { Grid, Typography } from '@mui/material';
import { Field, WrappedFieldProps } from 'redux-form';
import { WithStyles } from '@mui/styles';
import withStyles from '@mui/styles/withStyles';
import { VisibilityLevelSelect } from './visibility-level-select';
import { VisibilityLevel } from 'store/sharing-dialog/sharing-dialog-types';

const sharingPublicAccessStyles: CustomStyleRulesCallback<'root'> = theme => ({
    root: {
        padding: `${theme.spacing(2)} 0`,
    },
    heading: {
        fontSize: '1.25rem',
    }
});

interface AccessProps {
    visibility: VisibilityLevel;
    includePublic: boolean;
    onSave: () => void;
}

const SharingPublicAccessForm = withStyles(sharingPublicAccessStyles)(
    ({ classes, visibility, includePublic, onSave }: WithStyles<'root' | 'heading'> & AccessProps) =>
        <>
            <Typography className={classes.heading}>General access</Typography>
            <Grid container alignItems='center' className={classes.root}>
                <Grid item xs={8}>
                    <Typography variant='subtitle1'>
                        {renderVisibilityInfo(visibility)}
                    </Typography>
                </Grid>
                <Grid item xs={4} wrap='nowrap'>
                    <Field<{ includePublic: boolean }> name='visibility' component={VisibilityLevelSelectComponent} includePublic={includePublic} onChange={onSave} />
                </Grid>
            </Grid>
        </>
);

const renderVisibilityInfo = (visibility: VisibilityLevel) => {
    switch (visibility) {
        case VisibilityLevel.PUBLIC:
            return 'Shared with anyone on the Internet';
        case VisibilityLevel.ALL_USERS:
            return 'Shared with all users on this cluster';
        case VisibilityLevel.SHARED:
            return 'Shared with specific people';
        case VisibilityLevel.PRIVATE:
            return 'Not shared';
        default:
            return '';
    }
};

const SharingPublicAccessFormComponent = ({ visibility, includePublic, onSave }: AccessProps) =>
    <SharingPublicAccessForm {...{ visibility, includePublic, onSave }} />;

export default SharingPublicAccessFormComponent;

const VisibilityLevelSelectComponent = ({ input, includePublic }: { includePublic: boolean } & WrappedFieldProps) =>
    <VisibilityLevelSelect fullWidth disableUnderline includePublic={includePublic} {...input} />;
