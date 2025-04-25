// Sets up a DynamoDB table with input from the API Gateway and a stream to a Lambda function which exports metrics to Dynatrace.

package main

import (
	"log"
	"os"

	"github.com/aws/aws-cdk-go/awscdk/v2"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsdynamodb"
	"github.com/aws/aws-cdk-go/awscdk/v2/awslambda"
	"github.com/aws/aws-cdk-go/awscdk/v2/awssecretsmanager"
	"github.com/aws/aws-cdk-go/awscdkapigatewayv2alpha/v2"
	"github.com/aws/aws-cdk-go/awscdkapigatewayv2integrationsalpha/v2"
	"github.com/aws/aws-cdk-go/awscdklambdagoalpha/v2"
	"github.com/joho/godotenv"

	"github.com/aws/aws-cdk-go/awscdk/v2/awslambdaeventsources"
	"github.com/aws/constructs-go/constructs/v10"
	"github.com/aws/jsii-runtime-go"
)

const envVarName = "TABLE_NAME"

const createFunctionDir = "../create-function"
const exportFunctionDir = "../export-function"

type DynamoDBStreamsLambdaGolangStackProps struct {
	awscdk.StackProps
}

func NewDynamoDBStreamsLambdaGolangStack(scope constructs.Construct, id string, props *DynamoDBStreamsLambdaGolangStackProps) awscdk.Stack {
	// Load environment variables from .env file
	// This should include the Dynatrace environment name as 'DT_ENV'
	// For example: DT_ENV=your-dynatrace-env
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}

	var sprops awscdk.StackProps
	if props != nil {
		sprops = props.StackProps
	}
	stack := awscdk.NewStack(scope, &id, &sprops)

	// Create a DynamoDB table
	sourceDynamoDBTable := awsdynamodb.NewTable(stack, jsii.String("source-dynamodb-table"),
		&awsdynamodb.TableProps{
			PartitionKey: &awsdynamodb.Attribute{
				Name: jsii.String("email"),
				Type: awsdynamodb.AttributeType_STRING},
			Stream: awsdynamodb.StreamViewType_NEW_AND_OLD_IMAGES})

	sourceDynamoDBTable.ApplyRemovalPolicy(awscdk.RemovalPolicy_DESTROY)

	// Create a Lambda function to handle API requests and write to DynamoDB
	createUserFunction := awscdklambdagoalpha.NewGoFunction(stack, jsii.String("create-function"),
		&awscdklambdagoalpha.GoFunctionProps{
			Runtime:     awslambda.Runtime_PROVIDED_AL2(),
			Environment: &map[string]*string{envVarName: sourceDynamoDBTable.TableName()},
			Entry:       jsii.String(createFunctionDir)})

	sourceDynamoDBTable.GrantWriteData(createUserFunction)

	// Create and API Gateway for the create function
	api := awscdkapigatewayv2alpha.NewHttpApi(stack, jsii.String("http-api"), nil)

	createFunctionIntg := awscdkapigatewayv2integrationsalpha.NewHttpLambdaIntegration(jsii.String("create-function-integration"), createUserFunction, nil)

	api.AddRoutes(&awscdkapigatewayv2alpha.AddRoutesOptions{
		Path:        jsii.String("/"),
		Methods:     &[]awscdkapigatewayv2alpha.HttpMethod{awscdkapigatewayv2alpha.HttpMethod_POST},
		Integration: createFunctionIntg})

	// Create a Lambda function to handle DynamoDB stream events and export to Dynatrace
	exportFunction := awscdklambdagoalpha.NewGoFunction(stack, jsii.String("export-function"),
		&awscdklambdagoalpha.GoFunctionProps{
			Runtime:     awslambda.Runtime_PROVIDED_AL2(),
			Environment: &map[string]*string{"DynatraceEnv": jsii.String(os.Getenv("DT_ENV"))},
			Entry:       jsii.String(exportFunctionDir),
		})

	// Grant the export function read access to the Dynatrace token stored in Secrets Manager (set up in AWS Console)
	dtToken := awssecretsmanager.Secret_FromSecretCompleteArn(stack, jsii.String("DynatraceToken"),
		jsii.String("arn:aws:secretsmanager:eu-west-2:550844279723:secret:dynatrace-nonprod-yAabha"))

	dtToken.GrantRead(exportFunction, nil)

	// Connect the stream from the DynamoDB table to the export function
	exportFunction.AddEventSource(awslambdaeventsources.NewDynamoEventSource(sourceDynamoDBTable, &awslambdaeventsources.DynamoEventSourceProps{StartingPosition: awslambda.StartingPosition_LATEST, Enabled: jsii.Bool(true)}))

	// Print the API Gateway endpoint to the console
	awscdk.NewCfnOutput(stack, jsii.String("api-gateway-endpoint"),
		&awscdk.CfnOutputProps{
			ExportName: jsii.String("API-Gateway-Endpoint"),
			Value:      api.Url()})

	return stack
}

func main() {
	app := awscdk.NewApp(nil)

	NewDynamoDBStreamsLambdaGolangStack(app, "DynamoDBStreamsLambdaGolangStack", &DynamoDBStreamsLambdaGolangStackProps{
		awscdk.StackProps{
			Env: env(),
		},
	})

	app.Synth(nil)
}

// env determines the AWS environment (account+region) in which our stack is to
// be deployed. For more information see: https://docs.aws.amazon.com/cdk/latest/guide/environments.html
func env() *awscdk.Environment {
	return nil
}
