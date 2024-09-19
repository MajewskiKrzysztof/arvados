// Copyright (C) The Arvados Authors. All rights reserved.
//
// SPDX-License-Identifier: AGPL-3.0

import React from "react";
import { Dialog, DialogTitle, DialogContent, DialogActions, Button, Typography, Grid } from "@mui/material";
import { WithDialogProps } from "store/dialog/with-dialog";
import { withDialog } from 'store/dialog/with-dialog';
import { WithStyles } from '@mui/styles';
import withStyles from '@mui/styles/withStyles';
import { CustomStyleRulesCallback } from 'common/custom-theme';
import { ArvadosTheme } from 'common/custom-theme';
import { compose } from "redux";
import { GroupResource } from "models/group";
import { GROUP_ATTRIBUTES_DIALOG } from "store/groups-panel/groups-panel-actions";

type CssRules = 'rightContainer' | 'leftContainer' | 'spacing';

const styles: CustomStyleRulesCallback<CssRules> = (theme: ArvadosTheme) => ({
    rightContainer: {
        textAlign: 'right',
        paddingRight: theme.spacing(2),
        color: theme.palette.grey["500"]
    },
    leftContainer: {
        textAlign: 'left',
        paddingLeft: theme.spacing(2)
    },
    spacing: {
        paddingTop: theme.spacing(2)
    },
});

interface GroupAttributesDataProps {
    data: GroupResource;
}

type GroupAttributesProps = GroupAttributesDataProps & WithStyles<CssRules>;

export const GroupAttributesDialog = compose(
    withDialog(GROUP_ATTRIBUTES_DIALOG),
    withStyles(styles))(
        (props: WithDialogProps<GroupAttributesProps> & GroupAttributesProps) =>
            <Dialog open={props.open}
                onClose={props.closeDialog}
                fullWidth
                maxWidth="sm">
                <DialogTitle>Attributes</DialogTitle>
                <DialogContent>
                    <Typography variant='body1' className={props.classes.spacing}>
                        {props.data && attributes(props.data, props.classes)}
                    </Typography>
                </DialogContent>
                <DialogActions>
                    <Button
                        variant='text'
                        color='primary'
                        onClick={props.closeDialog}>
                        Close
                </Button>
                </DialogActions>
            </Dialog>
    );

const attributes = (group: GroupResource, classes: any) => {
    const { uuid, ownerUuid, createdAt, modifiedAt, modifiedByUserUuid, name, deleteAt, description, etag, href, isTrashed, trashAt} = group;
    return (
        <span>
            <Grid container direction="row">
                <Grid item xs={5} className={classes.rightContainer}>
                    {name && <Grid item>Name</Grid>}
                    {ownerUuid && <Grid item>Owner uuid</Grid>}
                    {createdAt && <Grid item>Created at</Grid>}
                    {modifiedAt && <Grid item>Modified at</Grid>}
                    {modifiedByUserUuid && <Grid item>Modified by user uuid</Grid>}
                    {uuid && <Grid item>uuid</Grid>}
                    {deleteAt && <Grid item>Delete at</Grid>}
                    {description && <Grid item>Description</Grid>}
                    {etag && <Grid item>Etag</Grid>}
                    {href && <Grid item>Href</Grid>}
                    {isTrashed && <Grid item>Is trashed</Grid>}
                    {trashAt && <Grid item>Trashed at</Grid>}
                </Grid>
                <Grid item xs={7} className={classes.leftContainer}>
                    <Grid item>{name}</Grid>
                    <Grid item>{ownerUuid}</Grid>
                    <Grid item>{createdAt}</Grid>
                    <Grid item>{modifiedAt}</Grid>
                    <Grid item>{modifiedByUserUuid}</Grid>
                    <Grid item>{uuid}</Grid>
                    <Grid item>{deleteAt}</Grid>
                    <Grid item>{description}</Grid>
                    <Grid item>{etag}</Grid>
                    <Grid item>{href}</Grid>
                    <Grid item>{isTrashed}</Grid>
                    <Grid item>{trashAt}</Grid>
                </Grid>
            </Grid>
        </span>
    );
};
