// Copyright (C) The Arvados Authors. All rights reserved.
//
// SPDX-License-Identifier: AGPL-3.0

import React from 'react';
import { CustomStyleRulesCallback } from 'common/custom-theme';
import { WithStyles } from '@mui/styles';
import withStyles from '@mui/styles/withStyles';
import { ArvadosTheme } from 'common/custom-theme';
import { Typography, Switch } from '@mui/material';

type CssRules = 'container' | 'label' | 'value';

const styles: CustomStyleRulesCallback<CssRules> = (theme: ArvadosTheme) => ({
    container: {
        display: 'flex',
        alignItems: 'center',
        height: '20px'
    },
    label: {
        width: '86px',
        color: theme.palette.grey["500"],
        textAlign: 'right',
    },
    value: {
        width: '24px',
        paddingLeft: theme.spacing(1),
    }
});

export interface SubprocessFilterDataProps {
    label: string;
    value: number;
    checked?: boolean;
    key?: string;
    onToggle?: () => void;
}

type SubprocessFilterProps = SubprocessFilterDataProps & WithStyles<CssRules>;

export const SubprocessFilter = withStyles(styles)(
    ({ classes, label, value, key, checked, onToggle }: SubprocessFilterProps) =>
        <div className={classes.container} >
            <Typography component="span" className={classes.label}>{label}:</Typography>
            <Typography component="span" className={classes.value}>{value}</Typography>
            {onToggle && <Switch
                checked={checked}
                onChange={onToggle}
                value={key}
                color="primary" />
            }
        </div>
);