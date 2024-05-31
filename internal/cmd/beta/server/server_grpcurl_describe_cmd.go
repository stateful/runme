package server

import (
	"context"
	"fmt"
	"strings"

	"github.com/fullstorydev/grpcurl"
	"github.com/jhump/protoreflect/desc"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
	"google.golang.org/protobuf/types/descriptorpb"

	"github.com/stateful/runme/v3/internal/config"
	"github.com/stateful/runme/v3/internal/config/autoconfig"
)

func serverGRPCurlDescribeCmd() *cobra.Command {
	cmd := cobra.Command{
		Use:   "describe",
		Short: "Describe gRPC services and methods exposed by the server.",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return autoconfig.InvokeForCommand(
				func(
					cfg *config.Config,
					logger *zap.Logger,
				) error {
					defer logger.Sync()

					result, err := describeSymbols(cmd.Context(), cfg, args...)
					if err != nil {
						return err
					}

					_, err = cmd.OutOrStdout().Write([]byte(strings.Join(result, "\n")))
					if err != nil {
						return errors.WithStack(err)
					}
					_, err = cmd.OutOrStdout().Write([]byte("\n"))
					return errors.WithStack(err)
				},
			)
		},
	}

	return &cmd
}

func describeSymbols(ctx context.Context, cfg *config.Config, symbols ...string) ([]string, error) {
	// If there are no symbols, we get all services.
	if len(symbols) == 0 {
		var err error
		symbols, err = listServices(ctx, cfg)
		if err != nil {
			return nil, err
		}
	}

	descSource, err := getDescriptorSource(ctx, cfg)
	if err != nil {
		return nil, err
	}

	var result []string

	for _, s := range symbols {
		if s[0] == '.' {
			s = s[1:]
		}

		dsc, err := descSource.FindSymbol(s)
		if err != nil {
			return nil, err
		}

		var elementType string
		switch d := dsc.(type) {
		case *desc.MessageDescriptor:
			elementType = "a message"
			parent, ok := d.GetParent().(*desc.MessageDescriptor)
			if ok {
				if d.IsMapEntry() {
					for _, f := range parent.GetFields() {
						if f.IsMap() && f.GetMessageType() == d {
							// found it: describe the map field instead
							elementType = "the entry type for a map field"
							dsc = f
							break
						}
					}
				} else {
					// see if it's a group
					for _, f := range parent.GetFields() {
						if f.GetType() == descriptorpb.FieldDescriptorProto_TYPE_GROUP && f.GetMessageType() == d {
							// found it: describe the map field instead
							elementType = "the type of a group field"
							dsc = f
							break
						}
					}
				}
			}
		case *desc.FieldDescriptor:
			elementType = "a field"
			if d.GetType() == descriptorpb.FieldDescriptorProto_TYPE_GROUP {
				elementType = "a group field"
			} else if d.IsExtension() {
				elementType = "an extension"
			}
		case *desc.OneOfDescriptor:
			elementType = "a one-of"
		case *desc.EnumDescriptor:
			elementType = "an enum"
		case *desc.EnumValueDescriptor:
			elementType = "an enum value"
		case *desc.ServiceDescriptor:
			elementType = "a service"
		case *desc.MethodDescriptor:
			elementType = "a method"
		default:
			return nil, errors.Errorf("descriptor has unrecognized type %T", dsc)
		}

		txt, err := grpcurl.GetDescriptorText(dsc, descSource)
		if err != nil {
			return nil, errors.Wrapf(err, "descriptor has unrecognized type %T", dsc)
		}

		result = append(result, fmt.Sprintf("%s is %s:\n%s", dsc.GetFullyQualifiedName(), elementType, txt))

		if dsc, ok := dsc.(*desc.MessageDescriptor); ok {
			// for messages, also show a template in JSON, to make it easier to
			// create a request to invoke an RPC
			tmpl := grpcurl.MakeTemplate(dsc)
			options := grpcurl.FormatOptions{EmitJSONDefaultFields: true}
			_, formatter, err := grpcurl.RequestParserAndFormatter(defaultGRPCurlFormat, descSource, nil, options)
			if err != nil {
				return nil, errors.Wrap(err, "failed to construct json formatter")
			}
			str, err := formatter(tmpl)
			if err != nil {
				return nil, errors.Wrapf(err, "failed to print template for message %s", s)
			}
			result = append(result, fmt.Sprintf("Message template: %s", str))
		}
	}

	return result, nil
}
