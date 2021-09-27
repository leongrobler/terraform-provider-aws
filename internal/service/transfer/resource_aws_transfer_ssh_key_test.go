package aws

import (
	"fmt"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/transfer"
	sdkacctest "github.com/hashicorp/terraform-plugin-sdk/v2/helper/acctest"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
	"github.com/hashicorp/terraform-provider-aws/internal/acctest"
	"github.com/hashicorp/terraform-provider-aws/internal/conns"
	"github.com/hashicorp/terraform-provider-aws/internal/provider"
	tftags "github.com/hashicorp/terraform-provider-aws/internal/tags"
	"github.com/hashicorp/terraform-provider-aws/internal/verify"
)

func testAccAWSTransferSshKey_basic(t *testing.T) {
	var conf transfer.SshPublicKey
	rName := sdkacctest.RandomWithPrefix("tf-acc-test")

	resourceName := "aws_transfer_ssh_key.test"

	publicKey, _, err := sdkacctest.RandSSHKeyPair(acctest.DefaultEmailAddress)
	if err != nil {
		t.Fatalf("error generating random SSH key: %s", err)
	}

	resource.Test(t, resource.TestCase{
		PreCheck:     func() { acctest.PreCheck(t); testAccPreCheckAWSTransfer(t) },
		ErrorCheck:   acctest.ErrorCheck(t, transfer.EndpointsID),
		Providers:    acctest.Providers,
		CheckDestroy: testAccCheckAWSTransferSshKeyDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccAWSTransferSshKeyConfig_basic(rName, publicKey),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckAWSTransferSshKeyExists(resourceName, &conf),
					resource.TestCheckResourceAttrPair(resourceName, "server_id", "aws_transfer_server.test", "id"),
					resource.TestCheckResourceAttrPair(resourceName, "user_name", "aws_transfer_user.test", "user_name"),
					resource.TestCheckResourceAttr(resourceName, "body", publicKey),
				),
			},
			{
				ResourceName:      resourceName,
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func testAccCheckAWSTransferSshKeyExists(n string, res *transfer.SshPublicKey) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[n]
		if !ok {
			return fmt.Errorf("Not found: %s", n)
		}

		if rs.Primary.ID == "" {
			return fmt.Errorf("No Transfer Ssh Public Key ID is set")
		}

		conn := acctest.Provider.Meta().(*conns.AWSClient).TransferConn
		serverID, userName, sshKeyID, err := decodeTransferSshKeyId(rs.Primary.ID)
		if err != nil {
			return fmt.Errorf("error parsing Transfer SSH Public Key ID: %s", err)
		}

		describe, err := conn.DescribeUser(&transfer.DescribeUserInput{
			ServerId: aws.String(serverID),
			UserName: aws.String(userName),
		})

		if err != nil {
			return err
		}

		for _, sshPublicKey := range describe.User.SshPublicKeys {
			if sshKeyID == *sshPublicKey.SshPublicKeyId {
				*res = *sshPublicKey
				return nil
			}
		}

		return fmt.Errorf("Transfer Ssh Public Key doesn't exists.")
	}
}

func testAccCheckAWSTransferSshKeyDestroy(s *terraform.State) error {
	conn := acctest.Provider.Meta().(*conns.AWSClient).TransferConn

	for _, rs := range s.RootModule().Resources {
		if rs.Type != "aws_transfer_ssh_key" {
			continue
		}
		serverID, userName, sshKeyID, err := decodeTransferSshKeyId(rs.Primary.ID)
		if err != nil {
			return fmt.Errorf("error parsing Transfer SSH Public Key ID: %w", err)
		}

		describe, err := conn.DescribeUser(&transfer.DescribeUserInput{
			UserName: aws.String(userName),
			ServerId: aws.String(serverID),
		})

		if tfawserr.ErrMessageContains(err, transfer.ErrCodeResourceNotFoundException, "") {
			continue
		}

		if err != nil {
			return err
		}

		for _, sshPublicKey := range describe.User.SshPublicKeys {
			if sshKeyID == *sshPublicKey.SshPublicKeyId {
				return fmt.Errorf("Transfer SSH Public Key still exists")
			}
		}
	}

	return nil
}

func testAccAWSTransferSshKeyConfig_basic(rName, publicKey string) string {
	return fmt.Sprintf(`
resource "aws_transfer_server" "test" {
  identity_provider_type = "SERVICE_MANAGED"
}

resource "aws_iam_role" "test" {
  name = %[1]q

  assume_role_policy = <<EOF
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Principal": {
        "Service": "transfer.amazonaws.com"
      },
      "Action": "sts:AssumeRole"
    }
  ]
}
EOF
}

resource "aws_iam_role_policy" "test" {
  name = %[1]q
  role = aws_iam_role.test.id

  policy = <<POLICY
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Sid": "AllowFullAccesstoS3",
      "Effect": "Allow",
      "Action": [
        "s3:*"
      ],
      "Resource": "*"
    }
  ]
}
POLICY
}

resource "aws_transfer_user" "test" {
  server_id = aws_transfer_server.test.id
  user_name = "tftestuser"
  role      = aws_iam_role.test.arn
}

resource "aws_transfer_ssh_key" "test" {
  server_id = aws_transfer_server.test.id
  user_name = aws_transfer_user.test.user_name
  body      = "%[2]s"
}
`, rName, publicKey)
}