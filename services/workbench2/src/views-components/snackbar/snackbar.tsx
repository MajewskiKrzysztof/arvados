// Copyright (C) The Arvados Authors. All rights reserved.
//
// SPDX-License-Identifier: AGPL-3.0

import React from "react";
import { Dispatch } from "redux";
import { connect } from "react-redux";
import { RootState } from "store/store";
import { CustomStyleRulesCallback } from 'common/custom-theme';
import { Button, IconButton, SnackbarContent } from '@mui/material';
import { WithStyles } from '@mui/styles';
import withStyles from '@mui/styles/withStyles';
import MaterialSnackbar, { SnackbarOrigin } from "@mui/material/Snackbar";
import { snackbarActions, SnackbarKind, SnackbarMessage } from "store/snackbar/snackbar-actions";
import { navigateTo } from 'store/navigation/navigation-action';
import WarningIcon from '@mui/icons-material/Warning';
import CheckCircleIcon from '@mui/icons-material/CheckCircle';
import ErrorIcon from '@mui/icons-material/Error';
import InfoIcon from '@mui/icons-material/Info';
import CloseIcon from '@mui/icons-material/Close';
import { ArvadosTheme } from "common/custom-theme";
import { amber, green } from "@mui/material/colors";
import classNames from 'classnames';

interface SnackbarDataProps {
    anchorOrigin?: SnackbarOrigin;
    autoHideDuration?: number;
    open: boolean;
    messages: SnackbarMessage[];
}

interface SnackbarEventProps {
    onClose?: (event: React.SyntheticEvent<any>, reason: string, message?: string) => void;
    onExited: () => void;
    onClick: (uuid: string) => void;
}

const mapStateToProps = (state: RootState): SnackbarDataProps => {
    const messages = state.snackbar.messages;
    return {
        anchorOrigin: { vertical: "bottom", horizontal: "right" },
        open: state.snackbar.open,
        messages,
        autoHideDuration: messages.length > 0 ? messages[0].hideDuration : 0
    };
};

const mapDispatchToProps = (dispatch: Dispatch): SnackbarEventProps => ({
    onClose: (event: any, reason: string, id: undefined) => {
        if (reason !== "clickaway") {
            dispatch(snackbarActions.CLOSE_SNACKBAR(id));
        }
    },
    onExited: () => {
        dispatch(snackbarActions.SHIFT_MESSAGES());
    },
    onClick: (uuid: string) => {
        dispatch<any>(navigateTo(uuid));
    }
});

type CssRules = "success" | "error" | "info" | "warning" | "icon" | "iconVariant" | "message" | "linkButton" | "snackbarContent";

const styles: CustomStyleRulesCallback<CssRules> = (theme: ArvadosTheme) => ({
    success: {
        backgroundColor: green[600]
    },
    error: {
        backgroundColor: theme.palette.error.dark
    },
    info: {
        backgroundColor: theme.palette.primary.main
    },
    warning: {
        backgroundColor: amber[700]
    },
    icon: {
        fontSize: 20
    },
    iconVariant: {
        opacity: 0.9,
        marginRight: theme.spacing(1)
    },
    message: {
        display: 'flex',
        alignItems: 'center'
    },
    linkButton: {
        fontWeight: 'bolder'
    },
    snackbarContent: {
        marginBottom: '1rem'
    }
});

type SnackbarProps = SnackbarDataProps & SnackbarEventProps & WithStyles<CssRules>;

export const Snackbar = withStyles(styles)(connect(mapStateToProps, mapDispatchToProps)(
    (props: SnackbarProps) => {
        const { classes } = props;

        const variants = {
            [SnackbarKind.INFO]: [InfoIcon, classes.info],
            [SnackbarKind.WARNING]: [WarningIcon, classes.warning],
            [SnackbarKind.SUCCESS]: [CheckCircleIcon, classes.success],
            [SnackbarKind.ERROR]: [ErrorIcon, classes.error]
        };

        return (
            <MaterialSnackbar
                TransitionProps={
                    {
                        onExited: props.onExited
                    }
                }
                open={props.open}
                onClose={props.onClose}
                anchorOrigin={props.anchorOrigin}
                autoHideDuration={props.autoHideDuration}>
                <div data-cy="snackbar">
                    {
                         props.messages.map((message, index) => {
                            const [Icon, cssClass] = variants[message.kind];

                            return <SnackbarContent
                                key={`${index}-${message.message}`}
                                className={classNames(cssClass, classes.snackbarContent)}
                                aria-describedby="client-snackbar"
                                message={
                                    <span id="client-snackbar" className={classes.message}>
                                        <Icon className={classNames(classes.icon, classes.iconVariant)} />
                                        {message.message}
                                    </span>
                                }
                                action={actions(message, props.onClick, props.onClose, classes, index, props.autoHideDuration)}
                            />
                         })
                    }
                </div>
            </MaterialSnackbar>
        );
    }
));

const actions = (props: SnackbarMessage, onClick, onClose, classes, index, autoHideDuration) => {
    if (onClose && autoHideDuration) {
        setTimeout(onClose, autoHideDuration);
    }

    const actions = [
        <IconButton
            key="close"
            aria-label="Close"
            color="inherit"
            onClick={e => onClose && onClose(e, '', index)}
            size="large">
            <CloseIcon className={classes.icon} />
        </IconButton>
    ];
    if (props.link) {
        actions.splice(0, 0,
            <Button key="goTo"
                aria-label="goTo"
                size="small"
                color="inherit"
                className={classes.linkButton}
                onClick={() => onClick(props.link)}>
                <span data-cy='snackbar-goto-action'>Go To</span>
            </Button>
        );
    }
    return actions;
};
