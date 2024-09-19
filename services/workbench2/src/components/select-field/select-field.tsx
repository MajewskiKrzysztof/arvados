// Copyright (C) The Arvados Authors. All rights reserved.
//
// SPDX-License-Identifier: AGPL-3.0

import React from 'react';
import { WrappedFieldProps } from 'redux-form';
import { ArvadosTheme } from 'common/custom-theme';
import { CustomStyleRulesCallback } from 'common/custom-theme';
import { FormControl, InputLabel, Select, FormHelperText } from '@mui/material';

import { WithStyles } from '@mui/styles';
import withStyles from '@mui/styles/withStyles';

type CssRules = 'formControl' | 'selectWrapper' | 'select' | 'option';

const styles: CustomStyleRulesCallback<CssRules> = (theme: ArvadosTheme) => ({
    formControl: {
        width: '100%',
    },
    selectWrapper: {
        backgroundColor: theme.palette.common.white,
        '&:before': {
            borderBottomColor: 'rgba(0, 0, 0, 0.42)',
        },
        '&:focus': {
            outline: 'none',
        },
    },
    select: {
        fontSize: '0.875rem',
        '&:focus': {
            backgroundColor: 'rgba(0, 0, 0, 0.0)',
        },
    },
    option: {
        fontSize: '0.875rem',
        backgroundColor: theme.palette.common.white,
        height: '30px',
    },
});

interface NativeSelectFieldProps {
    disabled?: boolean;
}

export const NativeSelectField = withStyles(styles)((props: WrappedFieldProps & NativeSelectFieldProps & WithStyles<CssRules> & { items: any[] }) => (
    <FormControl variant="standard" className={props.classes.formControl}>
        <Select
            variant="standard"
            className={props.classes.selectWrapper}
            native
            value={props.input.value}
            onChange={props.input.onChange}
            disabled={props.meta.submitting || props.disabled}
            name={props.input.name}
            inputProps={{
                id: `id-${props.input.name}`,
                className: props.classes.select,
            }}>
            {props.items.map(item => (
                <option
                    key={item.key}
                    value={item.key}
                    className={props.classes.option}>
                    {item.value}
                </option>
            ))}
        </Select>
    </FormControl>
));

interface SelectFieldProps {
    children: React.ReactNode;
    label: string;
}

type SelectFieldCssRules = 'formControl';

const selectFieldStyles: CustomStyleRulesCallback<SelectFieldCssRules> = (theme: ArvadosTheme) => ({
    formControl: {
        marginBottom: theme.spacing(3),
    },
});
export const SelectField = withStyles(selectFieldStyles)((props: WrappedFieldProps & SelectFieldProps & WithStyles<SelectFieldCssRules>) => (
    <FormControl
        variant="standard"
        error={props.meta.invalid}
        className={props.classes.formControl}>
        <InputLabel>{props.label}</InputLabel>
        <Select variant="standard" {...props.input}>{props.children}</Select>
        <FormHelperText>{props.meta.error}</FormHelperText>
    </FormControl>
));
