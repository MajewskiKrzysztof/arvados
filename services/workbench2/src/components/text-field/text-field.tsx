// Copyright (C) The Arvados Authors. All rights reserved.
//
// SPDX-License-Identifier: AGPL-3.0

import React from 'react';
import { WrappedFieldProps } from 'redux-form';
import { ArvadosTheme } from 'common/custom-theme';
import { CustomStyleRulesCallback } from 'common/custom-theme';
import { TextField as MaterialTextField, FormControlOwnProps } from '@mui/material';
import { WithStyles } from '@mui/styles';
import withStyles from '@mui/styles/withStyles';
import RichTextEditor from 'react-rte';

type CssRules = 'textField' | 'rte';

const styles: CustomStyleRulesCallback<CssRules> = (theme: ArvadosTheme) => ({
    textField: {
        marginBottom: theme.spacing(1)
    },
    rte: {
        fontFamily: 'Arial',
        '& a': {
            textDecoration: 'none',
            color: theme.palette.primary.main,
            '&:hover': {
                cursor: 'pointer',
                textDecoration: 'underline'
            }
        }
    }
});

type TextFieldProps = WrappedFieldProps & WithStyles<CssRules>;

export const TextField = withStyles(styles)((props: TextFieldProps & {
    label?: string, autoFocus?: boolean, required?: boolean, select?: boolean, disabled?: boolean, children: React.ReactNode, margin?: FormControlOwnProps["margin"], placeholder?: string,
    helperText?: string, type?: string,
}) =>
    <MaterialTextField
        variant="standard"
        helperText={(props.meta.touched && props.meta.error) || props.helperText}
        className={props.classes.textField}
        label={props.label}
        disabled={props.disabled || props.meta.submitting}
        error={props.meta.touched && !!props.meta.error}
        autoComplete='off'
        autoFocus={props.autoFocus}
        fullWidth={true}
        required={props.required}
        select={props.select}
        children={props.children}
        margin={props.margin}
        placeholder={props.placeholder}
        type={props.type}
        {...props.input} />);


interface RichEditorTextFieldData {
    label?: string;
}

type RichEditorTextFieldProps = RichEditorTextFieldData & TextFieldProps;

export const RichEditorTextField = withStyles(styles)(
    class RichEditorTextField extends React.Component<RichEditorTextFieldProps> {
        state = {
            value: RichTextEditor.createValueFromString(this.props.input.value, 'html')
        };

        onChange = (value: any) => {
            this.setState({ value });
            this.props.input.onChange(
                !!value.getEditorState().getCurrentContent().getPlainText().trim()
                ? value.toString('html')
                : null
            );
        }

        render() {
            return <RichTextEditor
                className={this.props.classes.rte}
                value={this.state.value}
                onChange={this.onChange}
                placeholder={this.props.label} />;
        }
    }
);

export const DateTextField = withStyles(styles)
    ((props: TextFieldProps) =>
        <MaterialTextField
            variant="standard"
            type="date"
            disabled={props.meta.submitting}
            helperText={props.meta.error}
            error={!!props.meta.error}
            fullWidth={true}
            InputLabelProps={{
                shrink: true
            }}
            name={props.input.name}
            onChange={props.input.onChange}
            value={props.input.value} />
    );
