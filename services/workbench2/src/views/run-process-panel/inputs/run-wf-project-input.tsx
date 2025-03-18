// Copyright (C) The Arvados Authors. All rights reserved.
//
// SPDX-License-Identifier: AGPL-3.0

import React from 'react';
import { connect, DispatchProp } from 'react-redux';
import { Field } from 'redux-form';
import { CustomStyleRulesCallback } from 'common/custom-theme';
import { Input, Dialog, DialogTitle, DialogContent, DialogActions, Button } from '@mui/material';
import { WithStyles } from '@mui/styles';
import withStyles from '@mui/styles/withStyles';
import {
    GenericCommandInputParameter
} from 'models/workflow';
import { GenericInput, GenericInputProps } from './generic-input';
import { ProjectsTreePicker } from 'views-components/projects-tree-picker/projects-tree-picker';
import { initProjectsTreePicker } from 'store/tree-picker/tree-picker-actions';
import { TreeItem } from 'components/tree/tree';
import { ProjectsTreePickerItem } from 'store/tree-picker/tree-picker-middleware';
import { ProjectResource } from 'models/project';
import { ResourceKind } from 'models/resource';
import { RootState } from 'store/store';
import { getUserUuid } from 'common/getuser';
import { getResource } from 'store/resources/resources';
import { loadProject } from 'store/workbench/workbench-actions';
import { runProcessPanelActions } from 'store/run-process-panel/run-process-panel-actions';

export type RunWfProjectCommandInputParameter = GenericCommandInputParameter<ProjectResource, ProjectResource>;

const isUndefined: any = (value?: ProjectResource) => (value === undefined);

interface ProjectInputProps {
    required: boolean;
    input: RunWfProjectCommandInputParameter;
    options?: { showOnlyOwned: boolean, showOnlyWritable: boolean };
}

type DialogContentCssRules = 'root' | 'pickerWrapper';

export const RunWfProjectInput = ({ required, input, options }: ProjectInputProps) =>
    <Field
        name={input.id}
        commandInput={input}
        component={ProjectInputComponent as any}
        format={format}
        validate={required ? isUndefined : undefined}
        {...{
            options,
            required
        }} />;

const format = (value?: ProjectResource) => value ? value.name : '';

interface ProjectInputComponentState {
    open: boolean;
    project?: ProjectResource;
    originalProject?: ProjectResource;
    hasBeenOpened: boolean;
}

type ProjectInputComponentProps = {
    userUuid: string | undefined;
    userRootProject: ProjectResource | undefined;
    targetProject: ProjectResource | undefined;
    defaultProject: ProjectResource | undefined;
    options?: { showOnlyOwned: boolean, showOnlyWritable: boolean };
    required?: boolean;
}

interface HasUserUuid {
    userUuid: string;
}

const mapStateToProps = (state: RootState): Pick<ProjectInputComponentProps, 'userUuid' | 'userRootProject' | 'targetProject' | 'defaultProject'> => {
    const userUuid = getUserUuid(state)
    const userRootProject = getResource<ProjectResource>(userUuid)(state.resources);
    const targetProject = getResource<ProjectResource>(state.runProcessPanel.processOwnerUuid)(state.resources)
    const defaultProject = targetProject || userRootProject;
    return {
        userUuid,
        userRootProject,
        targetProject,
        defaultProject,
    }
};

const ProjectInputComponent = connect(mapStateToProps)(
    class ProjectInputComponent extends React.Component<GenericInputProps & DispatchProp & HasUserUuid & ProjectInputComponentProps, ProjectInputComponentState> {

        state: ProjectInputComponentState = {
            open: false,
            project: undefined,
            originalProject: undefined,
            hasBeenOpened: false,
        };

        componentDidMount() {
            this.props.dispatch<any>(
                initProjectsTreePicker(this.props.commandInput.id));
            if (!this.state.project && this.props.defaultProject) {
                this.setState({ project: this.props.defaultProject, originalProject: this.props.defaultProject });
            }
            if (this.props.userUuid && !this.state.project) {
                this.props.dispatch<any>(loadProject(this.props.userUuid));
            }
            if (this.state.hasBeenOpened === false) {
                this.setState({ open: true, hasBeenOpened: true });
            }
        }

        componentDidUpdate(prevProps: any, prevState: ProjectInputComponentState) {
            if (prevProps.defaultProject !== this.props.defaultProject) {
                this.setState({ project: this.props.defaultProject });
            }
            if (!prevState.open && this.state.open) {
                this.setState({ project: this.props.defaultProject, originalProject: this.props.defaultProject });
            }
            if (!this.state.project && this.props.defaultProject) {
                this.setState({ project: this.props.defaultProject });
            }
            if (!this.props.targetProject && this.state.project) {
                this.props.dispatch<any>(runProcessPanelActions.SET_PROCESS_OWNER_UUID(this.state.project.uuid));
            }
        }

        componentWillUnmount(): void {
            this.props.dispatch<any>(runProcessPanelActions.SET_PROCESS_OWNER_UUID(''));
        }

        render() {
            return <>
                {this.renderInput()}
                <this.dialog />
            </>;
        }

        openDialog = () => {
            this.componentDidMount();
            this.setState({ open: true });
        }

        closeDialog = () => {
            this.setState({ open: false });
        }

        submit = () => {
            this.closeDialog();
            if (this.state.project) {
                if (this.state.project.kind === ResourceKind.PROJECT || this.state.project.kind === ResourceKind.USER) {
                    this.props.dispatch<any>(runProcessPanelActions.SET_PROCESS_OWNER_UUID(this.state.project.uuid));
                }
                if (this.state.originalProject && this.state.project.uuid !== this.state.originalProject.uuid) {
                    this.props.input.onChange(this.state.project);
                }
            }
        }

        setProject = (_: {}, { data }: TreeItem<ProjectsTreePickerItem>) => {
            if ('kind' in data){
                if (data.kind === ResourceKind.PROJECT) {
                    this.setState({ project: data });
                } else if (data.kind === ResourceKind.USER) {
                    this.setState({ project: this.props.userRootProject });
                }
            } else {
                this.setState({ project: undefined });
            }
        }

        getDisplayName(item: ProjectsTreePickerItem | undefined): string {
            if (item === undefined) {
                return '';
            }
            if ('kind' in item && item.kind === ResourceKind.USER) {
                return `${item.firstName} ${item.lastName} (root project)`;
            }
            if ('name' in item) {
                return item.name;
            } else {
                return '';
            }
        }


        invalid = () => (!this.state.project || !this.state.project.canWrite);

        renderInput() {
            return <GenericInput
                component={props =>
                    <Input
                        readOnly
                        fullWidth
                        value={props.input.value || this.getDisplayName(this.props.defaultProject)}
                        error={props.meta.touched && !!props.meta.error}
                        disabled={props.commandInput.disabled}
                        onClick={!this.props.commandInput.disabled ? this.openDialog : undefined}
                        onKeyPress={!this.props.commandInput.disabled ? this.openDialog : undefined} />}
                {...this.props} />;
        }

        dialogContentStyles: CustomStyleRulesCallback<DialogContentCssRules> = ({ spacing }) => ({
            root: {
                display: 'flex',
                flexDirection: 'column',
                height: "80vh",
            },
            pickerWrapper: {
                display: 'flex',
                flexDirection: 'column',
                height: "100%",
            },
        });

        dialog = withStyles(this.dialogContentStyles)(
            ({ classes }: WithStyles<DialogContentCssRules>) =>
                this.state.open ? <Dialog
                                      open={this.state.open}
                                      onClose={this.closeDialog}
                                      fullWidth
                                      data-cy="choose-a-project-dialog"
                                      maxWidth='md'>
                    <DialogTitle>Choose the project where the workflow will run</DialogTitle>
                    <DialogContent className={classes.root}>
                        <div className={classes.pickerWrapper}>
                            {this.state.project && <ProjectsTreePicker
                                pickerId={this.props.commandInput.id}
                                cascadeSelection={false}
                                options={this.props.options}
                                project={this.state.project}
                                currentUuids={[this.state.project.uuid]}
                                toggleItemActive={this.setProject} />}
                        </div>
                    </DialogContent>
                    <DialogActions>
                        <Button onClick={this.closeDialog}>Cancel</Button>
                        <Button
                            data-cy='run-wf-project-picker-ok-button'
                            disabled={this.invalid()}
                            variant='contained'
                            color='primary'
                            onClick={this.submit}>Ok</Button>
                    </DialogActions>
                </Dialog> : null
        );

    });
